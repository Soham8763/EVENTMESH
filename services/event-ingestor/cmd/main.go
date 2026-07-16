package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"eventmesh/event-ingestor/internal/api"
	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
	"eventmesh/event-ingestor/internal/producer"
	"eventmesh/event-ingestor/internal/ratelimit"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/pkg/tracing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	shutdown := tracing.Init("event-ingestor")
	defer shutdown()

	metrics.Init()

	// Expose Prometheus metrics on port 2112
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":2112", nil); err != nil {
			logger.Log.Error("metrics server failed", zap.Error(err))
		}
	}()
	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:8081"
	}
	authClient := auth.NewClient(authURL)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6380"
	}
	idempotencyStore := idempotency.NewStore(
		redisAddr,
		5*time.Minute,
	)

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	brokers := []string{kafkaBroker}
	if kafkaBroker == "" {
		brokers = []string{"127.0.0.1:19092"}
	}

	eventProducer, err := producer.NewProducer(
		brokers,
		"events",
	)
	if err != nil {
		logger.Log.Fatal("failed to create producer", zap.Error(err))
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	rateLimiter := ratelimit.NewLimiter(rdb)

	jwtValidator := auth.NewJWTValidator()

	handler := api.NewHandler(
		authClient,
		jwtValidator,
		idempotencyStore,
		rateLimiter,
		eventProducer,
	)

	http.HandleFunc("/events", handler.IngestEvent)

	srv := &http.Server{
		Addr: ":8080",
	}

	go func() {
		logger.Log.Info("event-ingestor running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal("service failure", zap.Error(err))
		}
	}()

	// Graceful shutdown handling
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig

	logger.Log.Info("event-ingestor shutting down...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("event-ingestor http shutdown failed", zap.Error(err))
	}
	
	// Close async producer to flush remaining messages
	if err := eventProducer.Close(); err != nil {
		logger.Log.Error("event-ingestor producer close failed", zap.Error(err))
	}

	logger.Log.Info("event-ingestor stopped")
}
