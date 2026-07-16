package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

type JWTValidator struct {
	secretKey []byte
}

type Claims struct {
	TenantID string `json:"tenant_id"`
	jwt.RegisteredClaims
}

func NewJWTValidator() *JWTValidator {
	secret := os.Getenv("JWT_SHARED_SECRET")
	if secret == "" {
		secret = "eventmesh-secret-key-change-me-in-production"
	}
	return &JWTValidator{
		secretKey: []byte(secret),
	}
}

func (v *JWTValidator) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.secretKey, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if claims.TenantID == "" {
			return "", errors.New("tenant_id claim is empty")
		}
		return claims.TenantID, nil
	}

	return "", errors.New("invalid token claims")
}
