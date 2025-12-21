package main

import (
	"log"
	"net/http"

	"eventmesh/event-ingestor/internal/api"
	"eventmesh/event-ingestor/internal/auth"
)

func main() {
	authClient := auth.NewClient("http://localhost:8081")
	handler := api.NewHandler(authClient)

	http.HandleFunc("/events", handler.IngestEvent)

	log.Println("event-ingestor running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
