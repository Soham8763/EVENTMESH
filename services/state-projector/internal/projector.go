package internal

import (
	"context"
	"log"

	"eventmesh/internal/events"
	"eventmesh/internal/kafka"

	"database/sql"
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
	log.Println("state-projector: starting consumer for", events.TopicExecutionEvents)
	for {
		msg, err := p.consumer.Read(ctx)
		if err != nil {
			log.Println("projection read error:", err)
			continue
		}

		p.handleEvent(msg.Value)
	}
}
