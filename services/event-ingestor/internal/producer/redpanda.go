package producer

import (
	"context"
	"encoding/json"
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

type Producer struct {
	producer sarama.AsyncProducer
	topic    string
}

func NewProducer(brokers []string, topic string) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Return.Successes = false // Not needed for async unless we track success channel
	cfg.Producer.Return.Errors = true
	cfg.Producer.Flush.Frequency = 10 * time.Millisecond // Batch for up to 10ms
	cfg.Producer.Flush.Messages = 100                     // Batch up to 100 messages

	p, err := sarama.NewAsyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}

	prod := &Producer{
		producer: p,
		topic:    topic,
	}

	go prod.handleErrors()

	return prod, nil
}

func NewProducerWithAsyncProducer(p sarama.AsyncProducer, topic string) *Producer {
	return &Producer{
		producer: p,
		topic:    topic,
	}
}

func (p *Producer) Publish(ctx context.Context, key string, value interface{}) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(bytes),
	}

	// Inject tracing context
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for k, v := range carrier {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	p.producer.Input() <- msg
	return nil
}

func (p *Producer) Close() error {
	return p.producer.Close()
}

func (p *Producer) handleErrors() {
	for err := range p.producer.Errors() {
		logger.Log.Error("async kafka produce error",
			zap.String("topic", err.Msg.Topic),
			zap.String("key", string(err.Msg.Key.(sarama.StringEncoder))),
			zap.Error(err.Err))
		metrics.EventsRejected.Inc()
	}
}
