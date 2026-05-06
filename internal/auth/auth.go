package auth

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	Email  string `json:"email"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Settings struct {
	Secret   string
	Issuer   string
	Audience string
}

var secretKey string
var issuer string
var audience string

func Init(s Settings) error {
	secretKey = s.Secret
	issuer = s.Issuer
	audience = s.Audience
	return nil
}

func ValidateConfig() error {
	if strings.TrimSpace(secretKey) == "" {
		return fmt.Errorf("JWT secret is not initialized")
	}
	if strings.TrimSpace(issuer) == "" {
		return fmt.Errorf("JWT issuer is not initialized")
	}
	if strings.TrimSpace(audience) == "" {
		return fmt.Errorf("JWT audience is not initialized")
	}
	return nil
}

func ValidateToken(tokenString string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString,
		claims,
		func(t *jwt.Token) (any, error) { return []byte(secretKey), nil },
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}
	return claims, nil
}
