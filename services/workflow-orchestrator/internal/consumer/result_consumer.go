package consumer

import (
	"context"
	"encoding/json"
	"log"

	"eventmesh/workflow-orchestrator/internal/model"

	"github.com/IBM/sarama"
)

type ResultHandler interface {
	HandleResult(model.TaskResult) error
}

type ResultConsumer struct {
	group  sarama.ConsumerGroup
	topic  string
	engine ResultHandler
}

func NewResultConsumer(
	brokers []string,
	groupID, topic string,
	engine ResultHandler,
) (*ResultConsumer, error) {

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	return &ResultConsumer{
		group:  group,
		topic:  topic,
		engine: engine,
	}, nil
}

func (c *ResultConsumer) Start(ctx context.Context) {
	log.Printf("result-consumer: starting for topic=%s", c.topic)
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			log.Printf("result-consumer error: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (c *ResultConsumer) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (c *ResultConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *ResultConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {

		var result model.TaskResult

		if err := json.Unmarshal(msg.Value, &result); err != nil {
			log.Printf("invalid result message: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		if err := c.engine.HandleResult(result); err != nil {
			log.Printf("result handling failed: %v", err)
			continue
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
