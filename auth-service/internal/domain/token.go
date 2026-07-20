package domain

import "github.com/golang-jwt/jwt/v5"

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenClaims struct {
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}
