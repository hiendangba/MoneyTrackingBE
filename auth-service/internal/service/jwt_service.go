package service

import (
	"errors"
	"fmt"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain"
	apperrors "auth-service/internal/errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTService struct {
	cfg config.JWTConfig
}

func NewJWTService(cfg config.JWTConfig) *JWTService {
	return &JWTService{cfg: cfg}
}

func (s *JWTService) GenerateAccessToken(userID string) (string, *domain.TokenClaims, error) {
	return s.generateToken(userID, domain.TokenTypeAccess, s.cfg.AccessTTL)
}

func (s *JWTService) GenerateRefreshToken(userID string) (string, *domain.TokenClaims, error) {
	return s.generateToken(userID, domain.TokenTypeRefresh, s.cfg.RefreshTTL)
}

func (s *JWTService) ParseAndValidate(tokenString string) (*domain.TokenClaims, error) {
	claims := &domain.TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return nil, apperrors.ErrInvalidToken
	}
	return claims, nil
}

func (s *JWTService) RemainingTTL(claims *domain.TokenClaims) time.Duration {
	if claims.ExpiresAt == nil {
		return 0
	}
	return time.Until(claims.ExpiresAt.Time)
}

func (s *JWTService) generateToken(userID, tokenType string, ttl time.Duration) (string, *domain.TokenClaims, error) {
	tokenID, err := uuid.NewV7()
	if err != nil {
		return "", nil, fmt.Errorf("generate token id: %w", err)
	}

	now := time.Now()
	claims := &domain.TokenClaims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID.String(),
			Subject:   userID,
			Issuer:    s.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", nil, fmt.Errorf("sign token: %w", err)
	}
	return signed, claims, nil
}

func ValidateTokenType(claims *domain.TokenClaims, want string) error {
	if claims == nil {
		return apperrors.ErrInvalidToken
	}
	if claims.TokenType != want {
		return fmt.Errorf("token type mismatch: %w", apperrors.ErrInvalidToken)
	}
	return nil
}

func TokenJTI(claims *domain.TokenClaims) (string, error) {
	if claims == nil || claims.ID == "" {
		return "", errors.New("token id missing")
	}
	return claims.ID, nil
}
