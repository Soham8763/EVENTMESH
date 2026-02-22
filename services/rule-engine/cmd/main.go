package main

import (
	"context"
	"os"
	"os/signal"

	"eventmesh/pkg/logger"
	"eventmesh/rule-engine/internal/consumer"
	"eventmesh/rule-engine/internal/matcher"
	"eventmesh/rule-engine/internal/producer"
	"eventmesh/rule-engine/internal/repository"

	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	logger.Log.Info("rule-engine starting...")

	db := repository.NewPostgres()
	ruleRepo := repository.NewRuleRepository(db)

	rules, err := ruleRepo.LoadActiveRules()
	if err != nil {
		logger.Log.Fatal("failed to load rules", zap.Error(err))
	}
	logger.Log.Info("loaded active rules", zap.Int("count", len(rules)))

	m := matcher.NewMatcher(rules)

	p, err := producer.NewProducer(
		[]string{"localhost:19092"},
		"workflow_triggers",
	)
	if err != nil {
		logger.Log.Fatal("failed to create producer", zap.Error(err))
	}

	eventConsumer, err := consumer.NewEventConsumer(
		[]string{"localhost:19092"},
		"rule-engine-group",
		"events",
		m,
		p,
	)
	if err != nil {
		logger.Log.Fatal("failed to create consumer", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go eventConsumer.Start(ctx)

	logger.Log.Info("rule-engine consuming from 'events' topic")

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig

	logger.Log.Info("rule-engine shutting down")
}
