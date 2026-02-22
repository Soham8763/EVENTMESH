package main

import (
	"net/http"
	"time"

	"eventmesh/event-ingestor/internal/api"
	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
	"eventmesh/event-ingestor/internal/producer"
	"eventmesh/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()
	authClient := auth.NewClient("http://localhost:8081")

	idempotencyStore := idempotency.NewStore(
		"localhost:6379",
		5*time.Minute,
	)

	eventProducer, err := producer.NewProducer(
		[]string{"localhost:19092"},
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
