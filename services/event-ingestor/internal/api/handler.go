package api

import (
	"encoding/json"
	"net/http"

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

	// 3. Decode request
	var req model.IngestEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.EventType == "" {
		http.Error(w, "event_type is required", http.StatusBadRequest)
		return
	}

	// 4. Accept event (no persistence yet)
	resp := model.IngestEventResponse{
		Status:   "accepted",
		TenantID: tenantID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
