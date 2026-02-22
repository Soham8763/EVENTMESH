package main

import (
	"net/http"

	"eventmesh/auth-service/internal/db"
	handler "eventmesh/auth-service/internal/http"
	"eventmesh/auth-service/internal/repository"
	"eventmesh/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	dbConn := db.NewPostgres()
	repo := repository.NewAPIKeyRepository(dbConn)
	h := handler.NewHandler(repo)

	http.HandleFunc("/validate", h.ValidateAPIKey)

	logger.Log.Info("auth-service running on :8081")
	logger.Log.Fatal("service failure", zap.Error(http.ListenAndServe(":8081", nil)))
}
