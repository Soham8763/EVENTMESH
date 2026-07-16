package events

import (
	"context"
	"encoding/json"
	"time"

	"eventmesh/internal/kafka"
)

type DLQMessage struct {
	OriginalTopic string    `json:"original_topic"`
	OriginalValue string    `json:"original_value"` // String representation of the raw payload
	Error         string    `json:"error"`
	Timestamp     time.Time `json:"timestamp"`
	Service       string    `json:"service"`
}

type DLQPublisher struct {
	producer *kafka.Producer
	service  string
}

func NewDLQPublisher(brokers []string, service string) *DLQPublisher {
	producer := kafka.NewProducer(brokers, "dead_letter_queue")
	return &DLQPublisher{
		producer: producer,
		service:  service,
	}
}

func (d *DLQPublisher) Publish(ctx context.Context, originalTopic string, rawValue []byte, err error) error {
	dlqMsg := DLQMessage{
		OriginalTopic: originalTopic,
		OriginalValue: string(rawValue),
		Error:         err.Error(),
		Timestamp:     time.Now().UTC(),
		Service:       d.service,
	}

	data, marshalErr := json.Marshal(dlqMsg)
	if marshalErr != nil {
		return marshalErr
	}

	return d.producer.Publish(ctx, "", data)
}
