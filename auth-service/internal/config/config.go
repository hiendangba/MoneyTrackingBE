package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	GRPC     GRPCServerConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	JWT      JWTConfig
	Auth     AuthConfig
	LogLevel slog.Level
}

type ServerConfig struct {
	Host string
	Port int
}

func (c ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type GRPCServerConfig struct {
	Host string
	Port int
}

func (c GRPCServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type PostgresConfig struct {
	Host     string
	Port     int
	DBName   string
	Username string
	Password string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type RabbitMQConfig struct {
	URL         string
	Exchange    string
	EmailQueue  string
	RegisterKey string
	ResetKey    string
}

type JWTConfig struct {
	PrivateKeyPath    string
	KeyID             string
	PublicKeyPaths    map[string]string
	AccessAudience    string
	RefreshAudience   string
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	Issuer            string
	AccessCookieName  string
	RefreshCookieName string
}

type AuthConfig struct {
	DefaultRoleCode    string
	AdminRoleCode      string
	OTPTTL             time.Duration
	OTPMaxAttempts     int
	CookieSecure       bool
	CookieSameSite     string
	CookieDomain       string
	CSRFCookieName     string
	AllowedOriginRegex string
}

func Load() (Config, error) {
	keyID := getEnv("JWT_KEY_ID", "money-tracking-auth-dev-key")
	publicKeyPath := getEnv("JWT_PUBLIC_KEY_PATH", "keys/jwt-public.pem")
	publicKeyPaths, err := parsePublicKeyPaths(getEnv("JWT_PUBLIC_KEYS", ""))
	if err != nil {
		return Config{}, err
	}
	if len(publicKeyPaths) == 0 {
		publicKeyPaths[keyID] = publicKeyPath
	}

	cfg := Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvInt("SERVER_PORT", 8080),
		},
		GRPC: GRPCServerConfig{
			Host: getEnv("GRPC_SERVER_HOST", "0.0.0.0"),
			Port: getEnvInt("GRPC_SERVER_PORT", 50051),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnvInt("POSTGRES_PORT", 5432),
			DBName:   getEnv("POSTGRES_DB", "postgres"),
			Username: getEnv("POSTGRES_USERNAME", "postgres"),
			Password: getEnv("POSTGRES_PASSWORD", ""),
			SSLMode:  getEnv("POSTGRES_SSL_MODE", "disable"),
			MaxConns: getEnvInt32("POSTGRES_MAX_CONNS", 10),
			MinConns: getEnvInt32("POSTGRES_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		RabbitMQ: RabbitMQConfig{
			URL:         getEnv("RABBITMQ_URL", ""),
			Exchange:    getEnv("RABBITMQ_EXCHANGE", "auth.events"),
			EmailQueue:  getEnv("RABBITMQ_EMAIL_QUEUE", "auth.email.otp"),
			RegisterKey: getEnv("RABBITMQ_REGISTER_KEY", "auth.otp.register"),
			ResetKey:    getEnv("RABBITMQ_RESET_KEY", "auth.otp.reset"),
		},
		JWT: JWTConfig{
			PrivateKeyPath:    getEnv("JWT_PRIVATE_KEY_PATH", "keys/jwt-private.pem"),
			KeyID:             keyID,
			PublicKeyPaths:    publicKeyPaths,
			AccessAudience:    getEnv("JWT_ACCESS_AUDIENCE", getEnv("JWT_AUDIENCE", "money-tracking-api")),
			RefreshAudience:   getEnv("JWT_REFRESH_AUDIENCE", "money-tracking-refresh"),
			AccessTTL:         getEnvDuration("JWT_ACCESS_TTL", 5*time.Minute),
			RefreshTTL:        getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			Issuer:            getEnv("JWT_ISSUER", "auth-service"),
			AccessCookieName:  getEnv("AUTH_ACCESS_COOKIE_NAME", "access_token"),
			RefreshCookieName: getEnv("AUTH_REFRESH_COOKIE_NAME", "refresh_token"),
		},
		Auth: AuthConfig{
			DefaultRoleCode:    getEnv("AUTH_DEFAULT_ROLE_CODE", "member"),
			AdminRoleCode:      getEnv("AUTH_ADMIN_ROLE_CODE", "admin"),
			OTPTTL:             getEnvDuration("AUTH_OTP_TTL", 5*time.Minute),
			OTPMaxAttempts:     getEnvInt("AUTH_OTP_MAX_ATTEMPTS", 5),
			CookieSecure:       getEnvBool("AUTH_COOKIE_SECURE", false),
			CookieSameSite:     getEnv("AUTH_COOKIE_SAME_SITE", "None"),
			CookieDomain:       getEnv("AUTH_COOKIE_DOMAIN", ""),
			CSRFCookieName:     getEnv("AUTH_CSRF_COOKIE_NAME", "csrf_token"),
			AllowedOriginRegex: getEnv("CORS_ALLOWED_ORIGIN_REGEX", `^https?://(localhost|127\.0\.0\.1)(:[0-9]+)?$`),
		},
		LogLevel: parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}

	var missing []string
	if cfg.Postgres.Username == "" {
		missing = append(missing, "POSTGRES_USERNAME")
	}
	if cfg.Postgres.Password == "" {
		missing = append(missing, "POSTGRES_PASSWORD")
	}
	if cfg.Redis.Password == "" {
		missing = append(missing, "REDIS_PASSWORD")
	}
	if cfg.RabbitMQ.URL == "" {
		missing = append(missing, "RABBITMQ_URL")
	}
	if cfg.JWT.PrivateKeyPath == "" {
		missing = append(missing, "JWT_PRIVATE_KEY_PATH")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	if cfg.JWT.KeyID == "" {
		return Config{}, errors.New("JWT_KEY_ID is required")
	}
	if cfg.JWT.AccessAudience == "" {
		return Config{}, errors.New("JWT_ACCESS_AUDIENCE is required")
	}
	if cfg.JWT.RefreshAudience == "" {
		return Config{}, errors.New("JWT_REFRESH_AUDIENCE is required")
	}
	if cfg.JWT.AccessAudience == cfg.JWT.RefreshAudience {
		return Config{}, errors.New("JWT access and refresh audiences must differ")
	}
	if cfg.JWT.Issuer == "" {
		return Config{}, errors.New("JWT_ISSUER is required")
	}
	if cfg.Auth.AdminRoleCode == "" {
		return Config{}, errors.New("AUTH_ADMIN_ROLE_CODE is required")
	}
	if cfg.JWT.AccessTTL <= 0 || cfg.JWT.RefreshTTL <= 0 {
		return Config{}, errors.New("JWT token TTLs must be positive")
	}
	if cfg.JWT.AccessTTL > 5*time.Minute {
		return Config{}, errors.New("JWT_ACCESS_TTL must not exceed 5 minutes")
	}
	if cfg.JWT.RefreshTTL <= cfg.JWT.AccessTTL {
		return Config{}, errors.New("JWT_REFRESH_TTL must exceed JWT_ACCESS_TTL")
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 || cfg.GRPC.Port <= 0 || cfg.GRPC.Port > 65535 || cfg.Postgres.Port <= 0 || cfg.Postgres.Port > 65535 {
		return Config{}, errors.New("server and postgres ports must be between 1 and 65535")
	}
	if cfg.Postgres.MaxConns <= 0 || cfg.Postgres.MinConns < 0 || cfg.Postgres.MinConns > cfg.Postgres.MaxConns {
		return Config{}, errors.New("invalid postgres connection pool limits")
	}
	if cfg.Auth.OTPMaxAttempts <= 0 || cfg.Auth.OTPTTL <= 0 {
		return Config{}, errors.New("OTP limits must be positive")
	}
	if cfg.JWT.AccessCookieName == "" || cfg.JWT.RefreshCookieName == "" || cfg.Auth.CSRFCookieName == "" {
		return Config{}, errors.New("auth cookie names are required")
	}
	if cfg.JWT.AccessCookieName == cfg.JWT.RefreshCookieName ||
		cfg.JWT.AccessCookieName == cfg.Auth.CSRFCookieName ||
		cfg.JWT.RefreshCookieName == cfg.Auth.CSRFCookieName {
		return Config{}, errors.New("auth cookie names must be distinct")
	}
	if strings.EqualFold(cfg.Auth.CookieSameSite, "none") && !cfg.Auth.CookieSecure {
		return Config{}, errors.New("AUTH_COOKIE_SECURE must be true when SameSite=None")
	}
	if cfg.Auth.AllowedOriginRegex == "" ||
		!strings.HasPrefix(cfg.Auth.AllowedOriginRegex, "^") ||
		!strings.HasSuffix(cfg.Auth.AllowedOriginRegex, "$") ||
		strings.Contains(cfg.Auth.AllowedOriginRegex, ".*") {
		return Config{}, errors.New("CORS_ALLOWED_ORIGIN_REGEX must be anchored and restrictive")
	}
	if _, err := regexp.Compile(cfg.Auth.AllowedOriginRegex); err != nil {
		return Config{}, fmt.Errorf("invalid CORS_ALLOWED_ORIGIN_REGEX: %w", err)
	}

	return cfg, nil
}

func parsePublicKeyPaths(raw string) (map[string]string, error) {
	paths := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return paths, nil
	}
	for _, entry := range strings.Split(raw, ";") {
		kid, path, ok := strings.Cut(strings.TrimSpace(entry), "=")
		kid = strings.TrimSpace(kid)
		path = strings.TrimSpace(path)
		if !ok || kid == "" || path == "" {
			return nil, fmt.Errorf("invalid JWT_PUBLIC_KEYS entry %q", entry)
		}
		if _, exists := paths[kid]; exists {
			return nil, fmt.Errorf("duplicate JWT public key id %q", kid)
		}
		paths[kid] = path
	}
	return paths, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvInt32(key string, fallback int32) int32 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(value) // #nosec G115 -- ParseInt enforces a signed 32-bit range.
}

func getEnvBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
