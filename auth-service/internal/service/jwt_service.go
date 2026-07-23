package service

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain"
	apperrors "auth-service/internal/errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const minimumRSAKeyBits = 2048
const jwtClockSkew = 30 * time.Second

type JWTService struct {
	cfg         config.JWTConfig
	privateKey  *rsa.PrivateKey
	publicKeys  map[string]*rsa.PublicKey
	jwksPayload []byte
}

func NewJWTService(cfg config.JWTConfig) (*JWTService, error) {
	privateKey, err := loadRSAPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load jwt private key: %w", err)
	}
	if err := validatePrivateKey(privateKey); err != nil {
		return nil, err
	}

	publicKeys := make(map[string]*rsa.PublicKey, len(cfg.PublicKeyPaths))
	for kid, path := range cfg.PublicKeyPaths {
		publicKey, loadErr := loadRSAPublicKey(path)
		if loadErr != nil {
			return nil, fmt.Errorf("load jwt public key %q: %w", kid, loadErr)
		}
		if publicKey.N.BitLen() < minimumRSAKeyBits {
			return nil, fmt.Errorf("jwt public key %q must be at least %d bits", kid, minimumRSAKeyBits)
		}
		publicKeys[kid] = publicKey
	}

	activePublicKey, ok := publicKeys[cfg.KeyID]
	if !ok {
		return nil, fmt.Errorf("active jwt key id %q is not present in JWT_PUBLIC_KEYS", cfg.KeyID)
	}
	if !privateKey.PublicKey.Equal(activePublicKey) {
		return nil, errors.New("active jwt private and public keys do not match")
	}

	jwksPayload, err := marshalJWKS(publicKeys)
	if err != nil {
		return nil, fmt.Errorf("marshal jwks: %w", err)
	}

	return &JWTService{
		cfg:         cfg,
		privateKey:  privateKey,
		publicKeys:  publicKeys,
		jwksPayload: jwksPayload,
	}, nil
}

func (s *JWTService) ParseAccessToken(tokenString string) (*domain.TokenClaims, error) {
	return s.parseAndValidate(tokenString, s.cfg.AccessAudience, domain.TokenTypeAccess)
}

func (s *JWTService) ParseRefreshToken(tokenString string) (*domain.TokenClaims, error) {
	return s.parseAndValidate(tokenString, s.cfg.RefreshAudience, domain.TokenTypeRefresh)
}

func (s *JWTService) parseAndValidate(tokenString, audience, tokenType string) (*domain.TokenClaims, error) {
	claims := &domain.TokenClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			kid, ok := token.Header["kid"].(string)
			if !ok || kid == "" {
				return nil, errors.New("token key id missing")
			}
			publicKey, ok := s.publicKeys[kid]
			if !ok {
				return nil, fmt.Errorf("unknown token key id %q", kid)
			}
			return publicKey, nil
		},
		jwt.WithAudience(audience),
		jwt.WithIssuer(s.cfg.Issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(jwtClockSkew),
	)
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid || !requiredClaimsPresent(claims) {
		return nil, apperrors.ErrInvalidToken
	}
	if err := validateTokenType(claims, tokenType); err != nil {
		return nil, err
	}
	if tokenType == domain.TokenTypeAccess && claims.RoleCode == "" {
		return nil, apperrors.ErrInvalidToken
	}
	if claims.ExpiresAt.Sub(claims.IssuedAt.Time) > tokenLifetime(s.cfg, tokenType)+jwtClockSkew {
		return nil, apperrors.ErrInvalidToken
	}
	return claims, nil
}

func tokenLifetime(cfg config.JWTConfig, tokenType string) time.Duration {
	if tokenType == domain.TokenTypeRefresh {
		return cfg.RefreshTTL
	}
	return cfg.AccessTTL
}

func requiredClaimsPresent(claims *domain.TokenClaims) bool {
	return claims != nil &&
		claims.ExpiresAt != nil &&
		claims.IssuedAt != nil &&
		claims.NotBefore != nil &&
		claims.Subject != "" &&
		claims.ID != "" &&
		claims.SessionID != "" &&
		claims.SessionVersion > 0
}

func (s *JWTService) JWKS() []byte {
	return append([]byte(nil), s.jwksPayload...)
}

func (s *JWTService) KeyID() string {
	return s.cfg.KeyID
}

func (s *JWTService) GenerateAccessToken(userID, roleID, roleCode, sessionID string, sessionVersion int64) (string, *domain.TokenClaims, error) {
	return s.generateToken(
		userID,
		roleID,
		roleCode,
		sessionID,
		sessionVersion,
		domain.TokenTypeAccess,
		s.cfg.AccessAudience,
		s.cfg.AccessTTL,
	)
}

func (s *JWTService) GenerateRefreshToken(userID, sessionID string, sessionVersion int64) (string, *domain.TokenClaims, error) {
	return s.generateToken(
		userID,
		"",
		"",
		sessionID,
		sessionVersion,
		domain.TokenTypeRefresh,
		s.cfg.RefreshAudience,
		s.cfg.RefreshTTL,
	)
}

func (s *JWTService) generateToken(
	userID, roleID, roleCode, sessionID string,
	sessionVersion int64,
	tokenType, audience string,
	ttl time.Duration,
) (string, *domain.TokenClaims, error) {
	tokenID, err := uuid.NewV7()
	if err != nil {
		return "", nil, fmt.Errorf("generate token id: %w", err)
	}

	now := time.Now()
	claims := &domain.TokenClaims{
		TokenType:      tokenType,
		SessionID:      sessionID,
		SessionVersion: sessionVersion,
		RoleID:         roleID,
		RoleCode:       roleCode,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID.String(),
			Subject:   userID,
			Issuer:    s.cfg.Issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.cfg.KeyID
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", nil, fmt.Errorf("sign token: %w", err)
	}
	return signed, claims, nil
}

func validatePrivateKey(key *rsa.PrivateKey) error {
	if key == nil {
		return errors.New("jwt private key is nil")
	}
	if key.N.BitLen() < minimumRSAKeyBits {
		return fmt.Errorf("jwt private key must be at least %d bits", minimumRSAKeyBits)
	}
	if err := key.Validate(); err != nil {
		return fmt.Errorf("validate jwt private key: %w", err)
	}
	return nil
}

func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	body, err := os.ReadFile(path) // #nosec G304 -- path is trusted operator configuration.
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(body)
	if block == nil {
		return nil, errors.New("invalid private key pem")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}
	parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
	if parseErr != nil {
		return nil, fmt.Errorf("parse private key: %w", parseErr)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not rsa")
	}
	return rsaKey, nil
}

func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	body, err := os.ReadFile(path) // #nosec G304 -- path is trusted operator configuration.
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(body)
	if block == nil {
		return nil, errors.New("invalid public key pem")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		certificate, certErr := x509.ParseCertificate(block.Bytes)
		if certErr != nil {
			return nil, fmt.Errorf("parse public key: %w", err)
		}
		rsaKey, ok := certificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("certificate public key is not rsa")
		}
		return rsaKey, nil
	}
	rsaKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not rsa")
	}
	return rsaKey, nil
}

func marshalJWKS(publicKeys map[string]*rsa.PublicKey) ([]byte, error) {
	type jwk struct {
		Kty string `json:"kty"`
		Use string `json:"use"`
		Kid string `json:"kid"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	type jwks struct {
		Keys []jwk `json:"keys"`
	}

	keyIDs := make([]string, 0, len(publicKeys))
	for kid := range publicKeys {
		keyIDs = append(keyIDs, kid)
	}
	sort.Strings(keyIDs)

	keys := make([]jwk, 0, len(keyIDs))
	for _, kid := range keyIDs {
		publicKey := publicKeys[kid]
		keys = append(keys, jwk{
			Kty: "RSA",
			Use: "sig",
			Kid: kid,
			Alg: jwt.SigningMethodRS256.Alg(),
			N:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(bigEndianBytes(publicKey.E)),
		})
	}

	payload, err := json.Marshal(jwks{Keys: keys})
	if err != nil {
		return nil, fmt.Errorf("marshal jwks payload: %w", err)
	}
	return payload, nil
}

func bigEndianBytes(value int) []byte {
	if value == 0 {
		return []byte{0}
	}
	var out []byte
	for value > 0 {
		out = append([]byte{byte(value & 0xff)}, out...)
		value >>= 8
	}
	return out
}

func (s *JWTService) RemainingTTL(claims *domain.TokenClaims) time.Duration {
	if claims == nil || claims.ExpiresAt == nil {
		return 0
	}
	return time.Until(claims.ExpiresAt.Time)
}

func validateTokenType(claims *domain.TokenClaims, want string) error {
	if claims == nil || claims.TokenType != want {
		return apperrors.ErrInvalidToken
	}
	return nil
}
