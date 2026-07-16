package producer

import (
	"context"
	"encoding/json"
	"time"

	"eventmesh/pkg/logger"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

type FailureEvent struct {
	Type          string `json:"type"`
	WorkflowID    string `json:"workflow_id,omitempty"`
	StepName      string `json:"step_name,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Reason        string `json:"reason"`
	Timestamp     string `json:"timestamp"`
}

type FailureProducer struct {
	producer sarama.SyncProducer
	topic    string
}

func NewFailureProducer(brokers []string) (*FailureProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	p, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &FailureProducer{producer: p, topic: "system_failures"}, nil
}

func NewFailureProducerWithSyncProducer(p sarama.SyncProducer) *FailureProducer {
	return &FailureProducer{producer: p, topic: "system_failures"}
}

func (fp *FailureProducer) EmitFailure(ctx context.Context, event FailureEvent) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339)

	bytes, err := json.Marshal(event)
	if err != nil {
		logger.Log.Error("failed to marshal failure event", zap.Error(err))
		return
	}

	msg := &sarama.ProducerMessage{
		Topic: fp.topic,
		Key:   sarama.StringEncoder(event.Type),
		Value: sarama.ByteEncoder(bytes),
	}

	_, _, err = fp.producer.SendMessage(msg)
	if err != nil {
		logger.Log.Error("failed to publish failure event", zap.Error(err))
	}
}
