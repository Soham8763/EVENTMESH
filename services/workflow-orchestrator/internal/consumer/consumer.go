package consumer

import (
	"context"
	"encoding/json"
	"log"

	"eventmesh/workflow-orchestrator/internal/model"

	"github.com/IBM/sarama"
)

type Engine interface {
	HandleTrigger(trigger model.WorkflowTriggerEvent) error
}

type TriggerConsumer struct {
	group  sarama.ConsumerGroup
	topic  string
	engine Engine
}

func NewTriggerConsumer(
	brokers []string,
	groupID, topic string,
	engine Engine,
) (*TriggerConsumer, error) {

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	return &TriggerConsumer{
		group:  group,
		topic:  topic,
		engine: engine,
	}, nil
}

func (c *TriggerConsumer) Start(ctx context.Context) {
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			log.Printf("consumer error: %v", err)
		}
	}
}

func (c *TriggerConsumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (c *TriggerConsumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (c *TriggerConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {

		var trigger model.WorkflowTriggerEvent

		if err := json.Unmarshal(msg.Value, &trigger); err != nil {
			log.Printf("invalid trigger message: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		if err := c.engine.HandleTrigger(trigger); err != nil {
			log.Printf("failed to create execution: %v", err)
			continue
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
