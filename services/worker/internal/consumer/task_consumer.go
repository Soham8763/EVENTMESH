package consumer

import (
	"context"
	"encoding/json"
	"log"

	"eventmesh/worker/internal/executor"
	"eventmesh/worker/internal/idempotency"
	"eventmesh/worker/internal/model"
	"eventmesh/worker/internal/producer"

	"github.com/IBM/sarama"
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
	log.Printf("task-consumer: starting for topic=%s", c.topic)
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			log.Printf("task-consumer error: %v", err)
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
		var task model.WorkflowTask
		if err := json.Unmarshal(msg.Value, &task); err != nil {
			log.Printf("invalid task message: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		// acquire idempotency lock
		lockKey := "task_lock:" + task.TaskID
		ok, err := c.store.Acquire(session.Context(), lockKey)
		if err != nil {
			log.Printf("idempotency error for task %s: %v", task.TaskID, err)
			continue
		}
		if !ok {
			log.Printf("duplicate task ignored: %s", task.TaskID)
			session.MarkMessage(msg, "")
			continue
		}

		// acquire lease
		leaseKey := "lease:" + task.TaskID
		ok, err = c.store.AcquireLease(session.Context(), leaseKey)
		if err != nil {
			log.Printf("lease error: %v", err)
			continue
		}
		if !ok {
			log.Printf("task already leased: %s", task.TaskID)
			continue
		}

		exec, err := c.registry.Get(task.StepName)
		if err != nil {
			log.Printf("error: %v", err)
			_ = c.store.ReleaseLease(session.Context(), leaseKey)
			session.MarkMessage(msg, "")
			continue
		}

		// execute task
		err = exec.Execute(task)

		// prepare result
		result := model.TaskResult{
			TaskID:              task.TaskID,
			WorkflowExecutionID: task.WorkflowExecutionID,
			StepName:            task.StepName,
		}

		if err != nil {
			log.Printf("execution failed for step %s: %v", task.StepName, err)
			errMsg := err.Error()
			result.Status = "FAILED"
			result.Error = &errMsg
		} else {
			result.Status = "SUCCESS"
		}

		// publish result
		log.Printf("publishing result %s for step %s", result.Status, result.StepName)
		if err := c.producer.Publish(task.WorkflowExecutionID, result); err != nil {
			log.Printf("failed to publish result: %v", err)
		}

		// release lease
		_ = c.store.ReleaseLease(session.Context(), leaseKey)

		session.MarkMessage(msg, "")
	}

	return nil
}
