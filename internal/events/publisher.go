package events

import (
	"context"

	"eventmesh/internal/kafka"
)

type EventPublisher struct {
	producer *kafka.Producer
}

func NewEventPublisher(brokers []string) *EventPublisher {
	producer := kafka.NewProducer(
		brokers,
		TopicExecutionEvents,
	)

	return &EventPublisher{
		producer: producer,
	}
}

func (p *EventPublisher) Publish(ctx context.Context, executionID string, event interface{}) error {
	data, err := Serialize(event)
	if err != nil {
		return err
	}

	return p.producer.Publish(ctx, executionID, data)
}
