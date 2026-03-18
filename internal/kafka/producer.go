package kafka

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) Publish(ctx context.Context, key string, value []byte) error {
	return p.PublishToTopic(ctx, "", key, value)
}

func (p *Producer) PublishToTopic(ctx context.Context, topic string, key string, value []byte) error {
	msg := kafka.Message{
		Key:   []byte(key),
		Value: value,
		Topic: topic,
	}

	err := p.writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Println("kafka publish error:", err)
	}

	return err
}
