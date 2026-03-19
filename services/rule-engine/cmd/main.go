package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"
	"eventmesh/rule-engine/internal/consumer"
	"eventmesh/rule-engine/internal/matcher"
	"eventmesh/rule-engine/internal/producer"
	"eventmesh/rule-engine/internal/repository"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	shutdown := tracing.Init("rule-engine")
	defer shutdown()

	metrics.Init()

	// Expose Prometheus metrics on port 2117
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2117", nil); err != nil {
			logger.Log.Error("metrics server failed", zap.Error(err))
		}
	}()

	logger.Log.Info("rule-engine starting...")

	db := repository.NewPostgres()
	ruleRepo := repository.NewRuleRepository(db)

	rules, err := ruleRepo.LoadActiveRules()
	if err != nil {
		logger.Log.Fatal("failed to load rules", zap.Error(err))
	}
	logger.Log.Info("loaded active rules", zap.Int("count", len(rules)))

	m := matcher.NewMatcher(rules)

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	brokers := []string{kafkaBroker}
	if kafkaBroker == "" {
		brokers = []string{"127.0.0.1:19092"}
	}

	p, err := producer.NewProducer(
		brokers,
		"workflow_triggers",
	)
	if err != nil {
		logger.Log.Fatal("failed to create producer", zap.Error(err))
	}

	eventConsumer, err := consumer.NewEventConsumer(
		brokers,
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
