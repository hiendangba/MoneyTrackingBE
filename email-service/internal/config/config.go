package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RabbitMQ RabbitMQConfig
	SMTP     SMTPConfig
	LogLevel slog.Level
}

type RabbitMQConfig struct {
	URL            string
	Exchange       string
	Queue          string
	RegisterKey    string
	ResetKey       string
	PrefetchCount  int
	ReconnectDelay time.Duration
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromAddr string
}

func Load() (Config, error) {
	cfg := Config{
		RabbitMQ: RabbitMQConfig{
			URL:            getEnv("RABBITMQ_URL", ""),
			Exchange:       getEnv("RABBITMQ_EXCHANGE", "auth.events"),
			Queue:          getEnv("RABBITMQ_EMAIL_QUEUE", "auth.email.otp"),
			RegisterKey:    getEnv("RABBITMQ_REGISTER_KEY", "auth.otp.register"),
			ResetKey:       getEnv("RABBITMQ_RESET_KEY", "auth.otp.reset"),
			PrefetchCount:  getEnvInt("RABBITMQ_PREFETCH_COUNT", 5),
			ReconnectDelay: getEnvDuration("RABBITMQ_RECONNECT_DELAY", 5*time.Second),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     getEnvInt("SMTP_PORT", 587),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("EMAIL_FROM", ""),
		},
		LogLevel: parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}

	var missing []string
	if cfg.RabbitMQ.URL == "" {
		missing = append(missing, "RABBITMQ_URL")
	}
	if cfg.SMTP.Host == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if cfg.SMTP.From == "" {
		missing = append(missing, "EMAIL_FROM")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	if cfg.SMTP.Port <= 0 || cfg.SMTP.Port > 65535 {
		return Config{}, errors.New("SMTP_PORT must be between 1 and 65535")
	}
	fromHeader, fromAddr, err := parseSender(cfg.SMTP.From)
	if err != nil {
		return Config{}, err
	}
	cfg.SMTP.From = fromHeader
	cfg.SMTP.FromAddr = fromAddr
	if cfg.RabbitMQ.PrefetchCount <= 0 {
		return Config{}, errors.New("RABBITMQ_PREFETCH_COUNT must be greater than 0")
	}

	return cfg, nil
}

func parseSender(raw string) (string, string, error) {
	sender, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("EMAIL_FROM must be a valid mailbox: %w", err)
	}
	return sender.String(), sender.Address, nil
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
