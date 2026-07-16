package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"
	"eventmesh/worker/internal/consumer"
	"eventmesh/worker/internal/executor"
	"eventmesh/worker/internal/idempotency"
	"eventmesh/worker/internal/producer"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	shutdown := tracing.Init("worker")
	defer shutdown()

	metrics.Init()

	// Expose Prometheus metrics on port 2114
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2114", nil); err != nil {
			logger.Log.Error("metrics server failed", zap.Error(err))
		}
	}()

	logger.Log.Info("worker starting...")

	// Worker heartbeat signal
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			metrics.WorkerHeartbeat.Set(float64(time.Now().Unix()))
			<-ticker.C
		}
	}()

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	brokers := []string{kafkaBroker}
	if kafkaBroker == "" {
		brokers = []string{"127.0.0.1:19092"}
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6380"
	}
	// Initialize idempotency store
	store := idempotency.NewStore(redisAddr, 10*time.Minute)

	// Initialize producer
	resProducer, err := producer.NewProducer(brokers, "workflow_task_results")
	if err != nil {
		logger.Log.Fatal("failed to create result producer", zap.Error(err))
	}

	// Initialize executors
	registry := executor.NewRegistry()
	registry.Register("send_email", &executor.SendEmailExecutor{})
	registry.Register("create_profile", &executor.CreateProfileExecutor{})

	// Define capabilities
	capabilities := []string{"send_email", "create_profile"}
	var taskTopics []string
	for _, cap := range capabilities {
		taskTopics = append(taskTopics, events.TaskTopic(cap))
	}

	// Initialize task consumer
	taskConsumer, err := consumer.NewTaskConsumer(
		brokers,
		"worker-group-4", // New group ID for fresh rebalance
		taskTopics,
		registry,
		resProducer,
		store,
	)
	if err != nil {
		logger.Log.Fatal("failed to create task consumer", zap.Error(err))
	}

	// Start consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go taskConsumer.Start(ctx)

	// Wait for terminate signal
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	<-sigterm

	logger.Log.Info("worker shutting down...")
	cancel()
	time.Sleep(500 * time.Millisecond)
	logger.Log.Info("worker stopped")
}
