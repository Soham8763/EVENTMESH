package internal

import (
	"context"

	"eventmesh/internal/events"
	"eventmesh/internal/kafka"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"

	"database/sql"

	"go.uber.org/zap"
)

type Projector struct {
	db       *sql.DB
	consumer *kafka.Consumer
}

func NewProjector(db *sql.DB, brokers []string) *Projector {

	consumer := kafka.NewConsumer(
		brokers,
		events.TopicExecutionEvents,
		"eventmesh-projector",
	)

	return &Projector{
		db:       db,
		consumer: consumer,
	}
}

func (p *Projector) Start(ctx context.Context) {
	logger.Log.Info("state-projector: starting consumer", zap.String("topic", events.TopicExecutionEvents))
	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("state-projector: shutting down")
			return
		default:
		}

		msg, err := p.consumer.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Log.Error("projection read error", zap.Error(err))
			continue
		}

		if err := p.handleEvent(msg.Value); err != nil {
			logger.Log.Error("projection handler error, message will be retried on next restart",
				zap.Error(err))
			// Do not commit offset — the message will be re-delivered
			continue
		}

		metrics.EventsProcessed.WithLabelValues("projected").Inc()
	}
}
