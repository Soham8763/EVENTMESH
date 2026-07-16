package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
	"eventmesh/event-ingestor/internal/model"
	"eventmesh/event-ingestor/internal/producer"
	"eventmesh/event-ingestor/internal/ratelimit"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	authClient       *auth.Client
	jwtValidator     *auth.JWTValidator
	idempotencyStore *idempotency.Store
	rateLimiter      *ratelimit.Limiter
	producer         *producer.Producer
}

func NewHandler(authClient *auth.Client, jwtValidator *auth.JWTValidator, idempotencyStore *idempotency.Store, rateLimiter *ratelimit.Limiter, producer *producer.Producer) *Handler {
	return &Handler{
		authClient:       authClient,
		jwtValidator:     jwtValidator,
		idempotencyStore: idempotencyStore,
		rateLimiter:      rateLimiter,
		producer:         producer,
	}
}

func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	// 0. Generate/Extract Request ID and Correlation ID
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}
	correlationID := r.Header.Get("X-Correlation-ID")
	if correlationID == "" {
		correlationID = uuid.New().String()
	}

	tr := otel.Tracer("event-ingestor")
	ctx, span := tr.Start(r.Context(), "IngestEvent", trace.WithAttributes(
		attribute.String("request_id", requestID),
		attribute.String("correlation_id", correlationID),
	))
	defer span.End()

	// 1. Authenticate (JWT first, fallback to API Key)
	var tenantID string
	var authErr error

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		tenantID, authErr = h.jwtValidator.ValidateToken(token)
	} else {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			metrics.EventsRejected.Inc()
			http.Error(w, "missing authentication token or API key", http.StatusUnauthorized)
			return
		}
		tenantID, authErr = h.authClient.ValidateAPIKey(apiKey)
	}

	if authErr != nil {
		metrics.EventsRejected.Inc()
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// 2. Check Rate Limit (Fail Open for Availability)
	allowed, err := h.rateLimiter.Allow(ctx, tenantID)
	if err != nil {
		logger.Log.Error("rate limiter error", zap.Error(err))
	} else if !allowed {
		metrics.EventsRejected.Inc()
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	metrics.EventsReceived.Inc()

	// 3. Decode + validate body
	var req model.IngestEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.EventsRejected.Inc()
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.EventType == "" {
		metrics.EventsRejected.Inc()
		http.Error(w, "event_type is required", http.StatusBadRequest)
		return
	}

	if req.Payload == nil {
		metrics.EventsRejected.Inc()
		http.Error(w, "payload is required", http.StatusBadRequest)
		return
	}

	// 4. Extract Idempotency-Key
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		metrics.EventsRejected.Inc()
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	// 5. Atomic idempotency check-and-set (SetNX)
	isNew, err := h.idempotencyStore.Acquire(ctx, idempotencyKey)
	if err != nil {
		logger.Log.Error("idempotency store error", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !isNew {
		// Duplicate event — safe to return OK (prevents retry storms)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", requestID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"duplicate","message":"event already processed"}`))
		return
	}

	// 6. Build enriched event
	envelope := model.EventEnvelope{
		EventID:        uuid.New().String(),
		EventType:      req.EventType,
		TenantID:       tenantID,
		CorrelationID:  correlationID,
		OccurredAt:     time.Now(),
		ReceivedAt:     time.Now(),
		RequestID:      requestID,
		IdempotencyKey: idempotencyKey,
		Payload:        req.Payload,
	}

	// 7. Publish envelope to Redpanda/Kafka
	if err := h.producer.Publish(ctx, tenantID, envelope); err != nil {
		logger.Log.Error("failed to publish event", zap.Error(err))
		// Rollback the idempotency key so the client can retry
		if releaseErr := h.idempotencyStore.Release(ctx, idempotencyKey); releaseErr != nil {
			logger.Log.Error("failed to release idempotency key", zap.Error(releaseErr))
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	logger.Log.Info("event published",
		zap.String("event_id", envelope.EventID),
		zap.String("tenant_id", tenantID),
		zap.String("correlation_id", correlationID))

	metrics.EventsProcessed.WithLabelValues("ingested").Inc()

	// 8. Return 202 Accepted
	resp := model.IngestEventResponse{
		Status:   "accepted",
		TenantID: tenantID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("X-Correlation-ID", correlationID)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}
