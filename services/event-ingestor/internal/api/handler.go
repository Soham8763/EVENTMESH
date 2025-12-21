package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
	"eventmesh/event-ingestor/internal/model"
)

type Handler struct {
	authClient       *auth.Client
	idempotencyStore *idempotency.Store
}

func NewHandler(authClient *auth.Client, idempotencyStore *idempotency.Store) *Handler {
	return &Handler{
		authClient:       authClient,
		idempotencyStore: idempotencyStore,
	}
}

func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	// 0. Generate/Extract Request ID (for tracing, logging, debugging)
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}
	ctx := r.Context()

	// 1. Extract API Key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "missing api key", http.StatusUnauthorized)
		return
	}

	// 2. Validate API Key
	tenantID, err := h.authClient.ValidateAPIKey(apiKey)
	if err != nil {
		http.Error(w, "invalid api key", http.StatusUnauthorized)
		return
	}

	// 3. Decode + validate body
	var req model.IngestEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.EventType == "" {
		http.Error(w, "event_type is required", http.StatusBadRequest)
		return
	}

	if req.Payload == nil {
		http.Error(w, "payload is required", http.StatusBadRequest)
		return
	}

	// 4. Extract Idempotency-Key
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	// 5. Check Redis (exists?)
	exists, err := h.idempotencyStore.Exists(ctx, idempotencyKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if exists {
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
		OccurredAt:     time.Now(),
		ReceivedAt:     time.Now(),
		RequestID:      requestID,
		IdempotencyKey: idempotencyKey,
		Payload:        req.Payload,
	}

	// TODO: Publish envelope to Redpanda/Kafka
	log.Printf("event accepted: %+v\n", envelope)

	// 7. Set Redis key (TTL)
	if err := h.idempotencyStore.Set(ctx, idempotencyKey); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 8. Return 200
	resp := model.IngestEventResponse{
		Status:   "accepted",
		TenantID: tenantID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	json.NewEncoder(w).Encode(resp)
}
