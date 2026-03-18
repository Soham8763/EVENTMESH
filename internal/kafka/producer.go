package kafka

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer       *kafka.Writer
	defaultTopic string
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Balancer: &kafka.LeastBytes{},
		},
		defaultTopic: topic,
	}
}

func (p *Producer) Publish(ctx context.Context, key string, value []byte) error {
	return p.PublishToTopic(ctx, "", key, value)
}

func (p *Producer) PublishToTopic(ctx context.Context, topic string, key string, value []byte) error {
	msg := kafka.Message{
		Key:   []byte(key),
		Value: value,
	}

	activeTopic := topic
	if activeTopic == "" {
		activeTopic = p.defaultTopic
	}
	msg.Topic = activeTopic

	err := p.writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("kafka publish error (topic: %s, key: %s): %v\n", activeTopic, key, err)
	}

	return err
}
