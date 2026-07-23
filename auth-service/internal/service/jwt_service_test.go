package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTServiceValidatesContractsAndRotatedKeys(t *testing.T) {
	t.Parallel()

	jwtService, activeKey, rotatedKey := newTestJWTService(t)
	accessToken, _, err := jwtService.GenerateAccessToken("user-1", "role-1", "member", "session-1", 1)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if _, err := jwtService.ParseAccessToken(accessToken); err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}

	refreshToken, _, err := jwtService.GenerateRefreshToken("user-1", "session-1", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	if _, err := jwtService.ParseAccessToken(refreshToken); err == nil {
		t.Fatal("ParseAccessToken() accepted a refresh token")
	}

	rotatedClaims := validTestClaims(domain.TokenTypeAccess, "money-tracking-api")
	rotatedToken := signTestToken(t, rotatedClaims, rotatedKey, "rotated-key")
	if _, err := jwtService.ParseAccessToken(rotatedToken); err != nil {
		t.Fatalf("ParseAccessToken() rejected rotated key: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*domain.TokenClaims)
		kid    string
		key    *rsa.PrivateKey
	}{
		{
			name: "wrong issuer",
			mutate: func(claims *domain.TokenClaims) {
				claims.Issuer = "other-issuer"
			},
			kid: "active-key",
			key: activeKey,
		},
		{
			name: "wrong audience",
			mutate: func(claims *domain.TokenClaims) {
				claims.Audience = jwt.ClaimStrings{"money-tracking-refresh"}
			},
			kid: "active-key",
			key: activeKey,
		},
		{
			name: "wrong token type",
			mutate: func(claims *domain.TokenClaims) {
				claims.TokenType = domain.TokenTypeRefresh
			},
			kid: "active-key",
			key: activeKey,
		},
		{
			name: "missing expiration",
			mutate: func(claims *domain.TokenClaims) {
				claims.ExpiresAt = nil
			},
			kid: "active-key",
			key: activeKey,
		},
		{
			name: "expired",
			mutate: func(claims *domain.TokenClaims) {
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
			},
			kid: "active-key",
			key: activeKey,
		},
		{
			name: "future not before",
			mutate: func(claims *domain.TokenClaims) {
				claims.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Minute))
			},
			kid: "active-key",
			key: activeKey,
		},
		{
			name:   "unknown key id",
			mutate: func(_ *domain.TokenClaims) {},
			kid:    "unknown-key",
			key:    activeKey,
		},
		{
			name:   "wrong signing key",
			mutate: func(_ *domain.TokenClaims) {},
			kid:    "active-key",
			key:    mustGenerateRSAKey(t),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			claims := validTestClaims(domain.TokenTypeAccess, "money-tracking-api")
			tt.mutate(claims)
			token := signTestToken(t, claims, tt.key, tt.kid)
			if _, err := jwtService.ParseAccessToken(token); err == nil {
				t.Fatalf("ParseAccessToken() accepted %s", tt.name)
			}
		})
	}

	t.Run("wrong algorithm", func(t *testing.T) {
		claims := validTestClaims(domain.TokenTypeAccess, "money-tracking-api")
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		token.Header["kid"] = "active-key"
		signed, err := token.SignedString([]byte("not-an-rsa-key"))
		if err != nil {
			t.Fatalf("SignedString() error = %v", err)
		}
		if _, err := jwtService.ParseAccessToken(signed); err == nil {
			t.Fatal("ParseAccessToken() accepted HS256")
		}
	})
}

func newTestJWTService(t *testing.T) (*JWTService, *rsa.PrivateKey, *rsa.PrivateKey) {
	t.Helper()

	activeKey := mustGenerateRSAKey(t)
	rotatedKey := mustGenerateRSAKey(t)
	dir := t.TempDir()
	activePrivatePath := filepath.Join(dir, "active-private.pem")
	activePublicPath := filepath.Join(dir, "active-public.pem")
	rotatedPublicPath := filepath.Join(dir, "rotated-public.pem")
	writePrivateKey(t, activePrivatePath, activeKey)
	writePublicKey(t, activePublicPath, &activeKey.PublicKey)
	writePublicKey(t, rotatedPublicPath, &rotatedKey.PublicKey)

	jwtService, err := NewJWTService(config.JWTConfig{
		PrivateKeyPath:  activePrivatePath,
		KeyID:           "active-key",
		PublicKeyPaths:  map[string]string{"active-key": activePublicPath, "rotated-key": rotatedPublicPath},
		AccessAudience:  "money-tracking-api",
		RefreshAudience: "money-tracking-refresh",
		AccessTTL:       5 * time.Minute,
		RefreshTTL:      7 * 24 * time.Hour,
		Issuer:          "auth-service",
	})
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}
	return jwtService, activeKey, rotatedKey
}

func validTestClaims(tokenType, audience string) *domain.TokenClaims {
	now := time.Now()
	return &domain.TokenClaims{
		TokenType:      tokenType,
		SessionID:      "session-1",
		SessionVersion: 1,
		RoleID:         "role-1",
		RoleCode:       "member",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "token-1",
			Subject:   "user-1",
			Issuer:    "auth-service",
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
}

func signTestToken(t *testing.T, claims *domain.TokenClaims, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}

func mustGenerateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, minimumRSAKeyBits)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return key
}

func writePrivateKey(t *testing.T, path string, key *rsa.PrivateKey) {
	t.Helper()
	body := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("os.WriteFile(private key) error = %v", err)
	}
}

func writePublicKey(t *testing.T, path string, key *rsa.PublicKey) {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	body := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("os.WriteFile(public key) error = %v", err)
	}
}
