package consumer

import (
	"context"
	"encoding/json"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

type Engine interface {
	HandleTrigger(ctx context.Context, trigger events.WorkflowTriggerEvent) error
	HandleWorkflowDefined(ctx context.Context, event events.WorkflowDefinedEvent) error
}

type TriggerConsumer struct {
	group  sarama.ConsumerGroup
	topic  string
	engine Engine
}

func NewTriggerConsumer(
	brokers []string,
	groupID, topic string,
	engine Engine,
) (*TriggerConsumer, error) {

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	return &TriggerConsumer{
		group:  group,
		topic:  topic,
		engine: engine,
	}, nil
}

func (c *TriggerConsumer) Start(ctx context.Context) {
	logger.Log.Info("trigger-consumer starting", zap.String("topic", c.topic))
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			logger.Log.Error("trigger consumer error", zap.Error(err))
		}
	}
}

func (c *TriggerConsumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (c *TriggerConsumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (c *TriggerConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	logger.Log.Info("trigger-consumer: ConsumeClaim started",
		zap.String("topic", claim.Topic()),
		zap.Int32("partition", claim.Partition()),
		zap.Int64("initialOffset", claim.InitialOffset()))

	for msg := range claim.Messages() {
		logger.Log.Info("trigger-consumer: received message",
			zap.Int64("offset", msg.Offset),
			zap.String("key", string(msg.Key)))

		// Extract tracing context from headers
		carrier := propagation.MapCarrier{}
		for _, h := range msg.Headers {
			carrier[string(h.Key)] = string(h.Value)
		}
		ctx := otel.GetTextMapPropagator().Extract(session.Context(), carrier)

		var trigger events.WorkflowTriggerEvent

		if err := json.Unmarshal(msg.Value, &trigger); err != nil {
			logger.Log.Error("invalid trigger message", zap.Error(err))
			session.MarkMessage(msg, "")
			continue
		}

		logger.Log.Info("trigger-consumer: parsed trigger",
			zap.String("workflow", trigger.WorkflowName),
			zap.String("trigger_id", trigger.TriggerID))

		if err := c.engine.HandleTrigger(ctx, trigger); err != nil {
			logger.Log.Error("failed to handle trigger",
				zap.Error(err),
				zap.String("workflow", trigger.WorkflowName))
			session.MarkMessage(msg, "")
			continue
		}

		logger.Log.Info("trigger-consumer: trigger handled successfully",
			zap.String("workflow", trigger.WorkflowName))

		metrics.EventsProcessed.WithLabelValues("trigger").Inc()

		session.MarkMessage(msg, "")
	}

	return nil
}

type RegistryConsumer struct {
	group  sarama.ConsumerGroup
	topic  string
	engine Engine
}

func NewRegistryConsumer(
	brokers []string,
	groupID, topic string,
	engine Engine,
) (*RegistryConsumer, error) {

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	return &RegistryConsumer{
		group:  group,
		topic:  topic,
		engine: engine,
	}, nil
}

func (c *RegistryConsumer) Start(ctx context.Context) {
	logger.Log.Info("registry-consumer starting", zap.String("topic", c.topic))
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			logger.Log.Error("registry consumer error", zap.Error(err))
		}
	}
}

func (c *RegistryConsumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (c *RegistryConsumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (c *RegistryConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {
		var event events.WorkflowDefinedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			logger.Log.Error("invalid registration message", zap.Error(err))
			session.MarkMessage(msg, "")
			continue
		}

		if err := c.engine.HandleWorkflowDefined(session.Context(), event); err != nil {
			logger.Log.Error("failed to handle registration", zap.Error(err))
			continue
		}

		metrics.EventsProcessed.WithLabelValues("registration").Inc()

		session.MarkMessage(msg, "")
	}

	return nil
}
