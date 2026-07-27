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
	Server              ServerConfig
	DB                  DBConfig
	AuthServiceAddress  string
	GroupServiceAddress string
	UpstreamTimeout     time.Duration
	DefaultCurrency     string
	LogLevel            slog.Level
}

type ServerConfig struct {
	Host string
	Port int
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DBConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

func Load() (Config, error) {
	level := slog.LevelInfo
	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		if err := level.UnmarshalText([]byte(raw)); err != nil {
			return Config{}, fmt.Errorf("parse LOG_LEVEL: %w", err)
		}
	}
	port, err := readInt("SERVER_PORT", 50053)
	if err != nil {
		return Config{}, err
	}
	maxConns, err := readInt("POSTGRES_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	minConns, err := readInt("POSTGRES_MIN_CONNS", 2)
	if err != nil {
		return Config{}, err
	}
	maxLifetime, err := time.ParseDuration(readString("POSTGRES_MAX_CONN_LIFETIME", "30m"))
	if err != nil {
		return Config{}, fmt.Errorf("parse POSTGRES_MAX_CONN_LIFETIME: %w", err)
	}
	upstreamTimeout, err := time.ParseDuration(readString("UPSTREAM_TIMEOUT", "5s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse UPSTREAM_TIMEOUT: %w", err)
	}
	dbURL := readString("TRANSACTION_DATABASE_URL", readString("DATABASE_URL", ""))
	currency := strings.ToUpper(readString("DEFAULT_CURRENCY", "VND"))

	cfg := Config{
		Server: ServerConfig{Host: readString("SERVER_HOST", "0.0.0.0"), Port: port},
		DB: DBConfig{
			URL:             dbURL,
			MaxConns:        int32(maxConns),
			MinConns:        int32(minConns),
			MaxConnLifetime: maxLifetime,
		},
		AuthServiceAddress:  readString("AUTH_SERVICE_ADDR", "auth-service:50051"),
		GroupServiceAddress: readString("GROUP_SERVICE_ADDR", "group-service:50052"),
		UpstreamTimeout:     upstreamTimeout,
		DefaultCurrency:     currency,
		LogLevel:            level,
	}
	switch {
	case cfg.Server.Port <= 0 || cfg.Server.Port > 65535:
		return Config{}, errors.New("SERVER_PORT must be between 1 and 65535")
	case cfg.DB.URL == "":
		return Config{}, errors.New("TRANSACTION_DATABASE_URL is required")
	case cfg.DB.MaxConns <= 0 || cfg.DB.MinConns < 0 || cfg.DB.MinConns > cfg.DB.MaxConns:
		return Config{}, errors.New("invalid postgres pool limits")
	case cfg.AuthServiceAddress == "" || cfg.GroupServiceAddress == "":
		return Config{}, errors.New("AUTH_SERVICE_ADDR and GROUP_SERVICE_ADDR are required")
	case cfg.UpstreamTimeout <= 0:
		return Config{}, errors.New("UPSTREAM_TIMEOUT must be positive")
	case len(cfg.DefaultCurrency) != 3:
		return Config{}, errors.New("DEFAULT_CURRENCY must be a 3-letter code")
	}
	return cfg, nil
}

func readString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func readInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return value, nil
}
