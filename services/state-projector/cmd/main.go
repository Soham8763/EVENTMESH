package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"
	"eventmesh/services/state-projector/internal"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/lib/pq"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	shutdown := tracing.Init("state-projector")
	defer shutdown()

	metrics.Init()

	// Expose Prometheus metrics on port 2115
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2115", nil); err != nil {
			log.Printf("metrics server failed: %v", err)
		}
	}()

	dsn := "postgres://eventmesh:eventmesh@localhost:5432/eventmesh?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	projector := internal.NewProjector(
		db,
		[]string{"127.0.0.1:19092"},
	)

	log.Println("state-projector: starting...")
	projector.Start(context.Background())
}
