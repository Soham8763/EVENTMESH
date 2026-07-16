package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidator(t *testing.T) {
	secret := "test-secret-key-12345"
	os.Setenv("JWT_SHARED_SECRET", secret)
	defer os.Unsetenv("JWT_SHARED_SECRET")

	validator := NewJWTValidator()

	// 1. Generate a valid token
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		TenantID: "tenant-ABC",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// 2. Validate valid token
	tenantID, err := validator.ValidateToken(tokenString)
	if err != nil {
		t.Errorf("Expected token to be valid, got error: %v", err)
	}
	if tenantID != "tenant-ABC" {
		t.Errorf("Expected tenantID 'tenant-ABC', got: %s", tenantID)
	}

	// 3. Validate token with wrong secret
	wrongValidator := &JWTValidator{secretKey: []byte("wrong-secret")}
	_, err = wrongValidator.ValidateToken(tokenString)
	if err == nil {
		t.Error("Expected error when validating with wrong secret, got nil")
	}

	// 4. Validate expired token
	expiredClaims := &Claims{
		TenantID: "tenant-ABC",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, _ := expiredToken.SignedString([]byte(secret))

	_, err = validator.ValidateToken(expiredTokenString)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}

	// 5. Validate token with missing tenant_id claim
	missingClaims := &Claims{
		TenantID: "",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	missingToken := jwt.NewWithClaims(jwt.SigningMethodHS256, missingClaims)
	missingTokenString, _ := missingToken.SignedString([]byte(secret))

	_, err = validator.ValidateToken(missingTokenString)
	if err == nil {
		t.Error("Expected error for missing tenant_id claim, got nil")
	}
}
