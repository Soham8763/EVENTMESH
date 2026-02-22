package consumer

import (
	"context"
	"encoding/json"
	"log"

	"eventmesh/worker/internal/executor"
	"eventmesh/worker/internal/model"

	"github.com/IBM/sarama"
)

type TaskConsumer struct {
	group    sarama.ConsumerGroup
	topic    string
	registry *executor.Registry
}

func NewTaskConsumer(
	brokers []string,
	groupID, topic string,
	registry *executor.Registry,
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

		exec, err := c.registry.Get(task.StepName)
		if err != nil {
			log.Printf("error: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		if err := exec.Execute(task); err != nil {
			log.Printf("execution failed for step %s: %v", task.StepName, err)
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
