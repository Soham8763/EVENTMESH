package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"
	"eventmesh/workflow-orchestrator/internal/api"
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

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	brokers := []string{kafkaBroker}
	if kafkaBroker == "" {
		brokers = []string{"127.0.0.1:19092"}
	}

	taskProducer, err := producer.NewProducer(
		brokers,
		"workflow_tasks",
	)
	if err != nil {
		logger.Log.Fatal("failed to create task producer", zap.Error(err))
	}

	failureProducer, err := producer.NewFailureProducer(
		brokers,
	)
	if err != nil {
		logger.Log.Fatal("failed to create failure producer", zap.Error(err))
	}

	eventPublisher := events.NewEventPublisher(brokers)

	execEngine := engine.NewExecutionEngine(db, taskProducer, failureProducer, eventPublisher)

	// Start stuck workflow checker
	stuckChecker := monitor.NewStuckChecker(db, execEngine.AdvanceExecution)

	triggerConsumer, err := consumer.NewTriggerConsumer(
		brokers,
		"orchestrator-triggers-v2",
		"workflow_triggers",
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create trigger consumer", zap.Error(err))
	}

	resultConsumer, err := consumer.NewResultConsumer(
		brokers,
		"orchestrator-results-v2",
		"workflow_task_results",
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create result consumer", zap.Error(err))
	}

	registryConsumer, err := consumer.NewRegistryConsumer(
		brokers,
		"orchestrator-registry-v2",
		events.TopicWorkflowRegistrations,
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create registry consumer", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Management REST API
	apiHandler := api.NewHandler(db)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/workflows", apiHandler.HandleWorkflows)
	apiMux.HandleFunc("/workflows/", apiHandler.HandleWorkflows)
	apiMux.HandleFunc("/executions", apiHandler.HandleExecutions)
	apiMux.HandleFunc("/executions/", apiHandler.HandleExecutions)

	apiSrv := &http.Server{
		Addr:    ":8082",
		Handler: apiMux,
	}

	go func() {
		logger.Log.Info("orchestrator management API running on :8082")
		if err := apiSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("management API server failed", zap.Error(err))
		}
	}()

	go triggerConsumer.Start(ctx)
	go resultConsumer.Start(ctx)
	go registryConsumer.Start(ctx)
	go stuckChecker.Start(ctx)

	logger.Log.Info("orchestrator ready and consuming")

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig

	logger.Log.Info("orchestrator shutting down...")
	cancel()

	// Shutdown HTTP Server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("management API shutdown failed", zap.Error(err))
	}

	time.Sleep(500 * time.Millisecond) // Allow active claims to finish
	logger.Log.Info("orchestrator stopped")
}
