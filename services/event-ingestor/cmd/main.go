package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("event-ingestor starting on :8082")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	if err := http.ListenAndServe(":8082", nil); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}