package main

import (
	"context"
	"net/http"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"
	"eventmesh/workflow-orchestrator/internal/consumer"
	"eventmesh/workflow-orchestrator/internal/engine"
	"eventmesh/workflow-orchestrator/internal/monitor"
	"eventmesh/workflow-orchestrator/internal/producer"
	"eventmesh/workflow-orchestrator/internal/repository"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	shutdown := tracing.Init("workflow-orchestrator")
	defer shutdown()

	metrics.Init()

	// Expose Prometheus metrics on port 2113
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2113", nil); err != nil {
			logger.Log.Error("metrics server failed", zap.Error(err))
		}
	}()

	logger.Log.Info("workflow-orchestrator starting...")

	db := repository.NewPostgres()
	workflowRepo := repository.NewWorkflowRepository(db)

	defs, err := workflowRepo.LoadDefinitions()
	if err != nil {
		logger.Log.Fatal("failed to load workflow definitions", zap.Error(err))
	}

	logger.Log.Info("loaded workflow definitions", zap.Int("count", len(defs)))

	taskProducer, err := producer.NewProducer(
		[]string{"localhost:19092"},
		"workflow_tasks",
	)
	if err != nil {
		logger.Log.Fatal("failed to create task producer", zap.Error(err))
	}

	failureProducer, err := producer.NewFailureProducer(
		[]string{"localhost:19092"},
	)
	if err != nil {
		logger.Log.Fatal("failed to create failure producer", zap.Error(err))
	}

	eventPublisher := events.NewEventPublisher([]string{"localhost:19092"})

	execEngine := engine.NewExecutionEngine(db, taskProducer, failureProducer, eventPublisher)

	// Start stuck workflow checker
	stuckChecker := monitor.NewStuckChecker(db)

	triggerConsumer, err := consumer.NewTriggerConsumer(
		[]string{"localhost:19092"},
		"workflow-orchestrator-group",
		"workflow_triggers",
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create trigger consumer", zap.Error(err))
	}

	resultConsumer, err := consumer.NewResultConsumer(
		[]string{"localhost:19092"},
		"workflow-results-group",
		"workflow_task_results",
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create result consumer", zap.Error(err))
	}

	registryConsumer, err := consumer.NewRegistryConsumer(
		[]string{"localhost:19092"},
		"workflow-registry-group",
		events.TopicWorkflowRegistrations,
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create registry consumer", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go triggerConsumer.Start(ctx)
	go resultConsumer.Start(ctx)
	go registryConsumer.Start(ctx)
	go stuckChecker.Start(ctx)

	logger.Log.Info("orchestrator ready and consuming")
	select {}
}
