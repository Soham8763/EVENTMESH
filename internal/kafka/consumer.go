package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string, topic string, groupID string) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		GroupID: groupID,
		Topic:   topic,
	})

	return &Consumer{
		reader: r,
	}
}

func (c *Consumer) Read(ctx context.Context) (kafka.Message, error) {
	return c.reader.ReadMessage(ctx)
}
