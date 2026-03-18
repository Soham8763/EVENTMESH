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

type Engine interface {
	HandleTrigger(ctx context.Context, trigger model.WorkflowTriggerEvent) error
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

	for msg := range claim.Messages() {
		// Extract tracing context from headers
		carrier := propagation.MapCarrier{}
		for _, h := range msg.Headers {
			carrier[string(h.Key)] = string(h.Value)
		}
		ctx := otel.GetTextMapPropagator().Extract(session.Context(), carrier)

		var trigger model.WorkflowTriggerEvent

		if err := json.Unmarshal(msg.Value, &trigger); err != nil {
			logger.Log.Error("invalid trigger message", zap.Error(err))
			session.MarkMessage(msg, "")
			continue
		}

		if err := c.engine.HandleTrigger(ctx, trigger); err != nil {
			logger.Log.Error("failed to handle trigger", zap.Error(err))
			continue
		}

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

		session.MarkMessage(msg, "")
	}

	return nil
}
