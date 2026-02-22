package consumer

import (
	"context"
	"encoding/json"
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/worker/internal/executor"
	"eventmesh/worker/internal/idempotency"
	"eventmesh/worker/internal/model"
	"eventmesh/worker/internal/producer"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

type TaskConsumer struct {
	group    sarama.ConsumerGroup
	topic    string
	registry *executor.Registry
	producer *producer.Producer
	store    *idempotency.Store
}

func NewTaskConsumer(
	brokers []string,
	groupID, topic string,
	registry *executor.Registry,
	producer *producer.Producer,
	store *idempotency.Store,
) (*TaskConsumer, error) {

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	return &TaskConsumer{
		group:    group,
		topic:    topic,
		registry: registry,
		producer: producer,
		store:    store,
	}, nil
}

func (c *TaskConsumer) Start(ctx context.Context) {
	logger.Log.Info("task-consumer: starting", zap.String("topic", c.topic))
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			logger.Log.Error("task-consumer error", zap.Error(err))
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (c *TaskConsumer) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (c *TaskConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *TaskConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {
		// Extract tracing context from headers
		carrier := propagation.MapCarrier{}
		for _, h := range msg.Headers {
			carrier[string(h.Key)] = string(h.Value)
		}
		ctx := otel.GetTextMapPropagator().Extract(session.Context(), carrier)

		tr := otel.Tracer("worker")
		ctx, span := tr.Start(ctx, "ConsumeTask")

		var task model.WorkflowTask
		if err := json.Unmarshal(msg.Value, &task); err != nil {
			logger.Log.Error("invalid task message", zap.Error(err))
			span.End()
			session.MarkMessage(msg, "")
			continue
		}

		span.SetAttributes(
			attribute.String("task_id", task.TaskID),
			attribute.String("workflow_id", task.WorkflowExecutionID),
			attribute.String("step", task.StepName),
		)

		// acquire idempotency lock
		lockKey := "task_lock:" + task.TaskID
		ok, err := c.store.Acquire(session.Context(), lockKey)
		if err != nil {
			logger.Log.Error("idempotency error", zap.String("task_id", task.TaskID), zap.Error(err))
			continue
		}
		if !ok {
			logger.Log.Warn("duplicate task ignored", zap.String("task_id", task.TaskID))
			session.MarkMessage(msg, "")
			continue
		}

		// acquire lease
		leaseKey := "lease:" + task.TaskID
		ok, err = c.store.AcquireLease(session.Context(), leaseKey)
		if err != nil {
			logger.Log.Error("lease error", zap.String("task_id", task.TaskID), zap.Error(err))
			continue
		}
		if !ok {
			logger.Log.Info("task already leased", zap.String("task_id", task.TaskID))
			continue
		}

		exec, err := c.registry.Get(task.StepName)
		if err != nil {
			logger.Log.Error("executor not found", zap.String("step", task.StepName), zap.Error(err))
			_ = c.store.ReleaseLease(session.Context(), leaseKey)
			session.MarkMessage(msg, "")
			continue
		}

		// execute task
		start := time.Now()
		err = exec.Execute(ctx, task)
		duration := time.Since(start).Seconds()

		metrics.TasksProcessed.Inc()
		metrics.TaskDuration.Observe(duration)

		// prepare result
		result := model.TaskResult{
			TaskID:              task.TaskID,
			WorkflowExecutionID: task.WorkflowExecutionID,
			StepName:            task.StepName,
		}

		if err != nil {
			metrics.TaskFailures.Inc()
			logger.Log.Error("execution failed",
				zap.String("step", task.StepName),
				zap.String("task_id", task.TaskID),
				zap.Error(err))
			errMsg := err.Error()
			result.Status = "FAILED"
			result.Error = &errMsg
		} else {
			result.Status = "SUCCESS"
		}

		// publish result
		logger.Log.Info("publishing result",
			zap.String("status", result.Status),
			zap.String("step", result.StepName),
			zap.String("execution_id", task.WorkflowExecutionID))
		if err := c.producer.Publish(ctx, task.WorkflowExecutionID, result); err != nil {
			logger.Log.Error("failed to publish result", zap.Error(err))
		}

		// release lease
		_ = c.store.ReleaseLease(session.Context(), leaseKey)

		span.End()
		session.MarkMessage(msg, "")
	}

	return nil
}
