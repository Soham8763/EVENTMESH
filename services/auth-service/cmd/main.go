package main

import (
	"log"
	"net/http"

	"eventmesh/auth-service/internal/db"
	handler "eventmesh/auth-service/internal/http"
	"eventmesh/auth-service/internal/repository"
)

func main() {
	dbConn := db.NewPostgres()
	repo := repository.NewAPIKeyRepository(dbConn)
	h := handler.NewHandler(repo)

	http.HandleFunc("/validate", h.ValidateAPIKey)

	log.Println("auth-service running on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
