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
	Server              ServerConfig
	AuthServiceAddress  string
	GroupServiceAddress string
	RequestTimeout      time.Duration
	AccessTTL           time.Duration
	RefreshTTL          time.Duration
	AllowedOriginRegex  string
	CSRF                CSRFConfig
	Cookie              CookieConfig
	Auth                AuthConfig
	LogLevel            slog.Level
}

type ServerConfig struct {
	Host string
	Port int
}

func (c ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type CSRFConfig struct {
	CookieName string
}

type CookieConfig struct {
	Secure   bool
	SameSite string
	Domain   string
}

type AuthConfig struct {
	AdminRoleCode string
}

func Load() (Config, error) {
	level := slog.LevelInfo
	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		if err := level.UnmarshalText([]byte(raw)); err != nil {
			return Config{}, fmt.Errorf("parse LOG_LEVEL: %w", err)
		}
	}

	port, err := readInt("SERVER_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	requestTimeout, err := time.ParseDuration(readString("REQUEST_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse REQUEST_TIMEOUT: %w", err)
	}

	corsRegex := readString("CORS_ALLOWED_ORIGIN_REGEX", `^https?://(localhost|127[.]0[.]0[.]1)(:[0-9]+)?$`)
	if corsRegex == "" || !strings.HasPrefix(corsRegex, "^") || !strings.HasSuffix(corsRegex, "$") {
		return Config{}, errors.New("CORS_ALLOWED_ORIGIN_REGEX must be anchored")
	}
	if _, err := regexp.Compile(corsRegex); err != nil {
		return Config{}, fmt.Errorf("invalid CORS_ALLOWED_ORIGIN_REGEX: %w", err)
	}

	cfg := Config{
		Server: ServerConfig{
			Host: readString("SERVER_HOST", "0.0.0.0"),
			Port: port,
		},
		AuthServiceAddress:  readString("AUTH_SERVICE_ADDR", "auth-service:50051"),
		GroupServiceAddress: readString("GROUP_SERVICE_ADDR", "group-service:50052"),
		RequestTimeout:      requestTimeout,
		AccessTTL:           getDuration("AUTH_ACCESS_TTL", 5*time.Minute),
		RefreshTTL:          getDuration("AUTH_REFRESH_TTL", 7*24*time.Hour),
		AllowedOriginRegex:  corsRegex,
		CSRF: CSRFConfig{
			CookieName: readString("GATEWAY_CSRF_COOKIE_NAME", "csrf_token"),
		},
		Cookie: CookieConfig{
			Secure:   readBool("AUTH_COOKIE_SECURE", false),
			SameSite: readString("AUTH_COOKIE_SAME_SITE", "Lax"),
			Domain:   readString("AUTH_COOKIE_DOMAIN", ""),
		},
		Auth: AuthConfig{
			AdminRoleCode: readString("AUTH_ADMIN_ROLE_CODE", "admin"),
		},
		LogLevel: level,
	}

	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return Config{}, errors.New("SERVER_PORT must be between 1 and 65535")
	}
	if cfg.AuthServiceAddress == "" {
		return Config{}, errors.New("AUTH_SERVICE_ADDR is required")
	}
	if cfg.GroupServiceAddress == "" {
		return Config{}, errors.New("GROUP_SERVICE_ADDR is required")
	}
	if cfg.RequestTimeout <= 0 {
		return Config{}, errors.New("REQUEST_TIMEOUT must be positive")
	}
	if cfg.AccessTTL <= 0 || cfg.RefreshTTL <= 0 {
		return Config{}, errors.New("AUTH_ACCESS_TTL and AUTH_REFRESH_TTL must be positive")
	}
	if cfg.RefreshTTL <= cfg.AccessTTL {
		return Config{}, errors.New("AUTH_REFRESH_TTL must exceed AUTH_ACCESS_TTL")
	}
	if cfg.AllowedOriginRegex == "" {
		return Config{}, errors.New("CORS_ALLOWED_ORIGIN_REGEX is required")
	}
	if cfg.CSRF.CookieName == "" {
		return Config{}, errors.New("GATEWAY_CSRF_COOKIE_NAME is required")
	}
	if cfg.Cookie.SameSite == "" {
		return Config{}, errors.New("AUTH_COOKIE_SAME_SITE is required")
	}
	if cfg.Auth.AdminRoleCode == "" {
		return Config{}, errors.New("AUTH_ADMIN_ROLE_CODE is required")
	}
	return cfg, nil
}

func readString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func readBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
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

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
