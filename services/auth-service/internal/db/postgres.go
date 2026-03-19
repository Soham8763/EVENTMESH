package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func NewPostgres() *sql.DB {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		dsn = "postgres://eventmesh:eventmesh@localhost:5432/eventmesh?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	return db
}
