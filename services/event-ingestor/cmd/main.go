package main

import (
	"net/http"
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
	authClient := auth.NewClient("http://localhost:8081")

	idempotencyStore := idempotency.NewStore(
		"localhost:6380",
		5*time.Minute,
	)

	eventProducer, err := producer.NewProducer(
		[]string{"127.0.0.1:19092"},
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
