package consumer

import (
	"context"
	"encoding/json"
	"log"

	"eventmesh/rule-engine/internal/model"

	"github.com/IBM/sarama"
)

type EventConsumer struct {
	group sarama.ConsumerGroup
	topic string
}

func NewEventConsumer(brokers []string, groupID, topic string) (*EventConsumer, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	return &EventConsumer{
		group: group,
		topic: topic,
	}, nil
}

func (c *EventConsumer) Start(ctx context.Context) {
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			log.Printf("consumer error: %v", err)
		}
	}
}

func (c *EventConsumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (c *EventConsumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (c *EventConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {
		var event model.EventEnvelope

		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("failed to decode event: %v", err)
			session.MarkMessage(msg, "")
			continue
		}

		log.Printf(
			"received event: event_id=%s event_type=%s tenant_id=%s",
			event.EventID,
			event.EventType,
			event.TenantID,
		)

		session.MarkMessage(msg, "")
	}

	return nil
}
