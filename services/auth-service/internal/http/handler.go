package http

import (
	"encoding/json"
	"net/http"

	"eventmesh/auth-service/internal/repository"
)

type Handler struct {
	repo *repository.APIKeyRepository
}

func NewHandler(repo *repository.APIKeyRepository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) ValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "missing api key", http.StatusUnauthorized)
		return
	}

	tenantID, err := h.repo.GetTenantID(apiKey)
	if err != nil {
		http.Error(w, "invalid api key", http.StatusUnauthorized)
		return
	}

	resp := map[string]string{
		"tenant_id": tenantID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
