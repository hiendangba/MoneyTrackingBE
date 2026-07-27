package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	DB       DBConfig
	LogLevel slog.Level
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

	port, err := readInt("SERVER_PORT", 50052)
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

	dbURL := strings.TrimSpace(os.Getenv("GROUP_DATABASE_URL"))
	if dbURL == "" {
		dbURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dbURL == "" {
		return Config{}, fmt.Errorf("GROUP_DATABASE_URL is required")
	}

	return Config{
		Server: ServerConfig{
			Host: readString("SERVER_HOST", "0.0.0.0"),
			Port: port,
		},
		DB: DBConfig{
			URL:             dbURL,
			MaxConns:        int32(maxConns),
			MinConns:        int32(minConns),
			MaxConnLifetime: maxLifetime,
		},
		LogLevel: level,
	}, nil
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
