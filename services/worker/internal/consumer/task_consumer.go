package consumer

import (
	"context"
	"encoding/json"
	"time"

	"eventmesh/internal/events"
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
	topics   []string
	registry *executor.Registry
	producer *producer.Producer
	store    *idempotency.Store
	dlq      *events.DLQPublisher
}

func NewTaskConsumer(
	brokers []string,
	groupID string,
	topics []string,
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

	dlq := events.NewDLQPublisher(brokers, "worker")

	return &TaskConsumer{
		group:    group,
		topics:   topics,
		registry: registry,
		producer: producer,
		store:    store,
		dlq:      dlq,
	}, nil
}

func (c *TaskConsumer) Start(ctx context.Context) {
	logger.Log.Info("task-consumer: starting", zap.Strings("topics", c.topics))
	for {
		if err := c.group.Consume(ctx, c.topics, c); err != nil {
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
			c.dlq.Publish(ctx, msg.Topic, msg.Value, err)
			span.End()
			session.MarkMessage(msg, "")
			continue
		}

		span.SetAttributes(
			attribute.String("task_id", task.TaskID),
			attribute.String("workflow_id", task.WorkflowExecutionID),
			attribute.String("step", task.StepName),
			attribute.String("correlation_id", task.CorrelationID),
		)

		// 1. Check if this task was already completed successfully
		doneKey := "task_done:" + task.TaskID
		done, err := c.store.IsDone(session.Context(), doneKey)
		if err != nil {
			logger.Log.Error("completion check error", zap.String("task_id", task.TaskID), zap.Error(err))
			span.End()
			// Don't mark message — retry on next rebalance
			continue
		}
		if done {
			logger.Log.Info("task already completed, skipping", zap.String("task_id", task.TaskID))
			span.End()
			session.MarkMessage(msg, "")
			continue
		}

		// 2. Acquire lease (short TTL, auto-expires if worker crashes)
		leaseKey := "lease:" + task.TaskID
		ok, err := c.store.AcquireLease(session.Context(), leaseKey)
		if err != nil {
			logger.Log.Error("lease error", zap.String("task_id", task.TaskID), zap.Error(err))
			span.End()
			continue
		}
		if !ok {
			logger.Log.Info("task already leased by another worker", zap.String("task_id", task.TaskID))
			span.End()
			// Don't mark message — let lease expire and another worker retry
			continue
		}

		// 3. Look up executor
		exec, err := c.registry.Get(task.StepName)
		if err != nil {
			logger.Log.Error("executor not found", zap.String("step", task.StepName), zap.Error(err))
			_ = c.store.ReleaseLease(session.Context(), leaseKey)
			span.End()
			session.MarkMessage(msg, "")
			continue
		}

		// 4. Execute task
		start := time.Now()
		err = exec.Execute(ctx, task)
		duration := time.Since(start).Seconds()

		metrics.TasksProcessed.Inc()
		metrics.TaskDuration.Observe(duration)
		metrics.WorkerThroughput.WithLabelValues("worker-1", task.StepName).Inc()

		// 5. Prepare result
		result := model.TaskResult{
			TaskID:              task.TaskID,
			WorkflowExecutionID: task.WorkflowExecutionID,
			StepName:            task.StepName,
			CorrelationID:       task.CorrelationID,
		}

		if err != nil {
			metrics.TaskFailures.Inc()
			logger.Log.Error("execution failed",
				zap.String("step", task.StepName),
				zap.String("task_id", task.TaskID),
				zap.String("correlation_id", task.CorrelationID),
				zap.Error(err))
			errMsg := err.Error()
			result.Status = "FAILED"
			result.Error = &errMsg
			metrics.StepExecutions.WithLabelValues(task.StepName, "failed").Inc()
		} else {
			result.Status = "SUCCESS"
			// Mark task as permanently completed — prevents re-execution
			if markErr := c.store.MarkDone(session.Context(), doneKey); markErr != nil {
				logger.Log.Error("failed to mark task as done", zap.Error(markErr))
			}
			metrics.StepExecutions.WithLabelValues(task.StepName, "completed").Inc()
		}

		// 6. Publish result
		logger.Log.Info("publishing result",
			zap.String("status", result.Status),
			zap.String("step", result.StepName),
			zap.String("execution_id", task.WorkflowExecutionID),
			zap.String("correlation_id", task.CorrelationID))
		if err := c.producer.Publish(ctx, task.WorkflowExecutionID, result); err != nil {
			logger.Log.Error("failed to publish result", zap.Error(err))
		}

		// 7. Release lease and commit offset
		_ = c.store.ReleaseLease(session.Context(), leaseKey)

		span.End()
		session.MarkMessage(msg, "")
	}

	return nil
}
