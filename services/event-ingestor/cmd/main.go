package main

import (
	"net/http"
	"os"
	"time"

	"eventmesh/event-ingestor/internal/api"
	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
	"eventmesh/event-ingestor/internal/producer"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	shutdown := tracing.Init("event-ingestor")
	defer shutdown()

	metrics.Init()

	// Expose Prometheus metrics on port 2112
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2112", nil); err != nil {
			logger.Log.Error("metrics server failed", zap.Error(err))
		}
	}()
	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:8081"
	}
	authClient := auth.NewClient(authURL)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6380"
	}
	idempotencyStore := idempotency.NewStore(
		redisAddr,
		5*time.Minute,
	)

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	brokers := []string{kafkaBroker}
	if kafkaBroker == "" {
		brokers = []string{"127.0.0.1:19092"}
	}

	eventProducer, err := producer.NewProducer(
		brokers,
		"events",
	)
	if err != nil {
		logger.Log.Fatal("failed to create producer", zap.Error(err))
	}

	handler := api.NewHandler(
		authClient,
		idempotencyStore,
		eventProducer,
	)

	http.HandleFunc("/events", handler.IngestEvent)

	logger.Log.Info("event-ingestor running on :8080")
	logger.Log.Fatal("service failure", zap.Error(http.ListenAndServe(":8080", nil)))
}
