package domain

import "github.com/golang-jwt/jwt/v5"

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenClaims struct {
	TokenType      string `json:"token_type"`
	SessionID      string `json:"sid"`
	SessionVersion int64  `json:"session_version"`
	RoleID         string `json:"role_id,omitempty"`
	RoleCode       string `json:"role_code,omitempty"`
	jwt.RegisteredClaims
}
