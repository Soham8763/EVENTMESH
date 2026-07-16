package consumer

import (
	"context"
	"encoding/json"
	"time"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/rule-engine/internal/matcher"
	"eventmesh/rule-engine/internal/model"
	"eventmesh/rule-engine/internal/producer"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

type EventConsumer struct {
	group    sarama.ConsumerGroup
	topic    string
	matcher  *matcher.Matcher
	producer *producer.Producer
	dlq      *events.DLQPublisher
}

func NewEventConsumer(
	brokers []string,
	groupID, topic string,
	matcher *matcher.Matcher,
	producer *producer.Producer,
) (*EventConsumer, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_1_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}

	dlq := events.NewDLQPublisher(brokers, "rule-engine")

	return &EventConsumer{
		group:    group,
		topic:    topic,
		matcher:  matcher,
		producer: producer,
		dlq:      dlq,
	}, nil
}

func (c *EventConsumer) Start(ctx context.Context) {
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, c); err != nil {
			logger.Log.Error("event consumer error", zap.Error(err))
		}
	}
}

func (c *EventConsumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (c *EventConsumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (c *EventConsumer) ConsumeClaim(
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

		tr := otel.Tracer("rule-engine")
		ctx, span := tr.Start(ctx, "ProcessEvent")

		var event model.EventEnvelope

		if err := json.Unmarshal(msg.Value, &event); err != nil {
			logger.Log.Error("failed to decode event", zap.Error(err))
			c.dlq.Publish(ctx, msg.Topic, msg.Value, err)
			span.End()
			session.MarkMessage(msg, "")
			continue
		}

		span.SetAttributes(
			attribute.String("correlation_id", event.CorrelationID),
			attribute.String("event_type", event.EventType),
		)

		logger.Log.Info("received event",
			zap.String("event_id", event.EventID),
			zap.String("event_type", event.EventType),
			zap.String("tenant_id", event.TenantID),
			zap.String("correlation_id", event.CorrelationID))

		matches := c.matcher.Match(event)

		for _, match := range matches {
			trigger := model.WorkflowTriggerEvent{
				TriggerID:     uuid.New().String(),
				EventID:       match.EventID,
				TenantID:      match.TenantID,
				WorkflowName:  match.WorkflowName,
				CorrelationID: match.CorrelationID,
				TriggeredAt:   time.Now().UTC(),
			}

			if err := c.producer.Publish(ctx, trigger.TenantID, trigger); err != nil {
				logger.Log.Error("failed to emit trigger",
					zap.String("correlation_id", trigger.CorrelationID),
					zap.Error(err))
				continue
			}

			logger.Log.Info("emitted workflow trigger",
				zap.String("workflow", trigger.WorkflowName),
				zap.String("event_id", trigger.EventID),
				zap.String("correlation_id", trigger.CorrelationID))
		}

		span.End()
		session.MarkMessage(msg, "")
	}

	return nil
}
