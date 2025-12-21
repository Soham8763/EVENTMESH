package main

import (
	"log"
	"net/http"
	"time"

	"eventmesh/event-ingestor/internal/api"
	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
)

func main() {
	authClient := auth.NewClient("http://localhost:8081")

	idempotencyStore := idempotency.NewStore(
		"localhost:6379",
		5*time.Minute,
	)

	handler := api.NewHandler(authClient, idempotencyStore)

	http.HandleFunc("/events", handler.IngestEvent)

	log.Println("event-ingestor running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
