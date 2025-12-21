package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/model"
)

type Handler struct {
	authClient *auth.Client
}

func NewHandler(authClient *auth.Client) *Handler {
	return &Handler{authClient: authClient}
}

func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	// 1. Generate/Extract Request ID (for tracing, logging, debugging)
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// 2. Extract Idempotency Key (required for deduplication - logic in Stage 1.4)
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	// 3. Extract API Key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "missing api key", http.StatusUnauthorized)
		return
	}

	// 4. Validate API Key
	tenantID, err := h.authClient.ValidateAPIKey(apiKey)
	if err != nil {
		http.Error(w, "invalid api key", http.StatusUnauthorized)
		return
	}

	// 5. Decode request
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

	// 6. Build event envelope
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

	// 7. Send response
	resp := model.IngestEventResponse{
		Status:   "accepted",
		TenantID: tenantID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	json.NewEncoder(w).Encode(resp)
}
