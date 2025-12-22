package main

import (
	"log"
	"net/http"
	"time"

	"eventmesh/event-ingestor/internal/api"
	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
	"eventmesh/event-ingestor/internal/producer"
)

func main() {
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
		log.Fatalf("failed to create producer: %v", err)
	}

	handler := api.NewHandler(
		authClient,
		idempotencyStore,
		eventProducer,
	)

	http.HandleFunc("/events", handler.IngestEvent)

	log.Println("event-ingestor running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
