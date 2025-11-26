package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
}

type Service struct {
	secretKey            []byte
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
}

// NewService creates a new JWT service
func NewService(secretKey string, accessDuration, refreshDuration time.Duration) *Service {
	return &Service{
		secretKey:            []byte(secretKey),
		accessTokenDuration:  accessDuration,
		refreshTokenDuration: refreshDuration,
	}
}

// ValidateToken validates and parses the JWT token
func (s *Service) ValidateToken(tokenStirng string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStirng, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secretKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	if claims.UserID == uuid.Nil {
		return nil, fmt.Errorf("invalid access token: missing user_id (this might be a refresh token)")
	}

	if claims.Email == "" {
		return nil, fmt.Errorf("invalid access token: missing email")
	}

	if claims.Username == "" {
		return nil, fmt.Errorf("invalid access token: missing username")
	}

	return claims, nil
}
