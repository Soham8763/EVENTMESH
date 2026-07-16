package main

import (
	"net/http"

	"eventmesh/auth-service/internal/db"
	handler "eventmesh/auth-service/internal/http"
	"eventmesh/auth-service/internal/repository"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	shutdown := tracing.Init("auth-service")
	defer shutdown()

	metrics.Init()

	// Expose Prometheus metrics on port 2116
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2116", nil); err != nil {
			logger.Log.Error("metrics server failed", zap.Error(err))
		}
	}()

	dbConn := db.NewPostgres()
	repo := repository.NewAPIKeyRepository(dbConn)
	h := handler.NewHandler(repo)

	http.HandleFunc("/validate", h.ValidateAPIKey)
	http.HandleFunc("/token", h.IssueToken)

	logger.Log.Info("auth-service running on :8081")
	logger.Log.Fatal("service failure", zap.Error(http.ListenAndServe(":8081", nil)))
}
