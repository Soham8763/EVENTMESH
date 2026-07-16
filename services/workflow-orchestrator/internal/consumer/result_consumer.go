package consumer

import (
	"context"
	"encoding/json"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/workflow-orchestrator/internal/model"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

type ResultHandler interface {
	HandleResult(ctx context.Context, result model.TaskResult) error
}

type ResultConsumer struct {
	group  sarama.ConsumerGroup
	topic  string
	engine ResultHandler
	dlq    *events.DLQPublisher
}

func NewResultConsumer(
	brokers []string,
	groupID, topic string,
	engine ResultHandler,
) (*ResultConsumer, error) {

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	dlq := events.NewDLQPublisher(brokers, "orchestrator-results")

	return &ResultConsumer{
		group:  group,
		topic:  topic,
		engine: engine,
		dlq:    dlq,
	}, nil
}

func (c *ResultConsumer) Start(ctx context.Context) {
	logger.Log.Info("result-consumer starting", zap.String("topic", c.topic))
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			logger.Log.Error("result-consumer error", zap.Error(err))
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (c *ResultConsumer) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (c *ResultConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *ResultConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {
		// Extract tracing context from headers
		carrier := propagation.MapCarrier{}
		for _, h := range msg.Headers {
			carrier[string(h.Key)] = string(h.Value)
		}
		ctx := otel.GetTextMapPropagator().Extract(session.Context(), carrier)

		var result model.TaskResult

		if err := json.Unmarshal(msg.Value, &result); err != nil {
			logger.Log.Error("invalid result message", zap.Error(err))
			c.dlq.Publish(ctx, msg.Topic, msg.Value, err)
			session.MarkMessage(msg, "")
			continue
		}

		if err := c.engine.HandleResult(ctx, result); err != nil {
			logger.Log.Error("result handling failed", zap.Error(err))
			continue
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
