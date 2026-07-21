package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
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
	RegisterKey string
	ResetKey    string
}

type JWTConfig struct {
	Secret            string
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	Issuer            string
	AccessCookieName  string
	RefreshCookieName string
}

type AuthConfig struct {
	DefaultRoleCode string
	OTPTTL          time.Duration
	OTPMaxAttempts  int
	CookieSecure    bool
	CookieSameSite  string
	CookieDomain    string
}

func Load() (Config, error) {
	cfg := Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvInt("SERVER_PORT", 8080),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnvInt("POSTGRES_PORT", 5432),
			DBName:   getEnv("POSTGRES_DB", "postgres"),
			Username: getEnv("POSTGRES_USERNAME", "postgres"),
			Password: getEnv("POSTGRES_PASSWORD", ""),
			SSLMode:  getEnv("POSTGRES_SSL_MODE", "disable"),
			MaxConns: int32(getEnvInt("POSTGRES_MAX_CONNS", 10)),
			MinConns: int32(getEnvInt("POSTGRES_MIN_CONNS", 5)),
		},
		// Redis: RedisConfig{
		// 	Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		// 	Password: getEnv("REDIS_PASSWORD", ""),
		// 	DB:       getEnvInt("REDIS_DB", 0),
		// },
		// RabbitMQ: RabbitMQConfig{
		// 	URL:         getEnv("RABBITMQ_URL", ""),
		// 	Exchange:    getEnv("RABBITMQ_EXCHANGE", "auth.events"),
		// 	RegisterKey: getEnv("RABBITMQ_REGISTER_KEY", "auth.otp.register"),
		// 	ResetKey:    getEnv("RABBITMQ_RESET_KEY", "auth.otp.reset"),
		// },
		JWT: JWTConfig{
			Secret:            getEnv("JWT_SECRET", ""),
			AccessTTL:         getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL:        getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			Issuer:            getEnv("JWT_ISSUER", "auth-service"),
			AccessCookieName:  getEnv("AUTH_ACCESS_COOKIE_NAME", "access_token"),
			RefreshCookieName: getEnv("AUTH_REFRESH_COOKIE_NAME", "refresh_token"),
		},
		Auth: AuthConfig{
			DefaultRoleCode: getEnv("AUTH_DEFAULT_ROLE_CODE", "member"),
			OTPTTL:          getEnvDuration("AUTH_OTP_TTL", 5*time.Minute),
			OTPMaxAttempts:  getEnvInt("AUTH_OTP_MAX_ATTEMPTS", 5),
			CookieSecure:    getEnvBool("AUTH_COOKIE_SECURE", false),
			CookieSameSite:  getEnv("AUTH_COOKIE_SAME_SITE", "None"),
			CookieDomain:    getEnv("AUTH_COOKIE_DOMAIN", ""),
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
	// if cfg.RabbitMQ.URL == "" {
	// 	missing = append(missing, "RABBITMQ_URL")
	// }
	if cfg.JWT.Secret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	if len(cfg.JWT.Secret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 characters")
	}

	return cfg, nil
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
