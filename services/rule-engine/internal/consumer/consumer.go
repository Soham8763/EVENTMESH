package consumer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"eventmesh/rule-engine/internal/matcher"
	"eventmesh/rule-engine/internal/model"
	"eventmesh/rule-engine/internal/producer"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

type EventConsumer struct {
	group    sarama.ConsumerGroup
	topic    string
	matcher  *matcher.Matcher
	producer *producer.Producer
}

func NewEventConsumer(
	brokers []string,
	groupID, topic string,
	matcher *matcher.Matcher,
	producer *producer.Producer,
) (*EventConsumer, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	return &EventConsumer{
		group:    group,
		topic:    topic,
		matcher:  matcher,
		producer: producer,
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

		matches := c.matcher.Match(event)

		for _, match := range matches {
			trigger := model.WorkflowTriggerEvent{
				TriggerID:    uuid.New().String(),
				EventID:      match.EventID,
				TenantID:     match.TenantID,
				WorkflowName: match.WorkflowName,
				TriggeredAt:  time.Now().UTC(),
			}

			if err := c.producer.Publish(trigger.TenantID, trigger); err != nil {
				log.Printf("failed to emit trigger: %v", err)
				continue
			}

			log.Printf(
				"emitted workflow trigger: workflow=%s event_id=%s",
				trigger.WorkflowName,
				trigger.EventID,
			)
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
