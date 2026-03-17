package main

import (
	"context"
	"database/sql"
	"log"

	"eventmesh/services/state-projector/internal"

	_ "github.com/lib/pq"
)

func main() {
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
		[]string{"localhost:19092"},
	)

	log.Println("state-projector: starting...")
	projector.Start(context.Background())
}
