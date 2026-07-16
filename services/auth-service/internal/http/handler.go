package http

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"eventmesh/auth-service/internal/repository"
	"github.com/golang-jwt/jwt/v5"
)

type Handler struct {
	repo      *repository.APIKeyRepository
	secretKey []byte
}

func NewHandler(repo *repository.APIKeyRepository) *Handler {
	secret := os.Getenv("JWT_SHARED_SECRET")
	if secret == "" {
		secret = "eventmesh-secret-key-change-me-in-production"
	}
	return &Handler{
		repo:      repo,
		secretKey: []byte(secret),
	}
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

type Claims struct {
	TenantID string `json:"tenant_id"`
	jwt.RegisteredClaims
}

func (h *Handler) IssueToken(w http.ResponseWriter, r *http.Request) {
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

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(h.secretKey)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"token":      tokenString,
		"expires_in": int(24 * time.Hour / time.Second),
		"token_type": "Bearer",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
