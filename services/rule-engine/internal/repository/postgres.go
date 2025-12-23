package repository

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func NewPostgres() *sql.DB {
	dsn := "postgres://eventmesh:eventmesh@localhost:5432/eventmesh?sslmode=disable"

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	return db
}
