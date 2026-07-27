package main

import (
	"fmt"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/config"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func bootstrap() (config.Config, *slog.Logger, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return config.Config{}, nil, fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	return cfg, logger, nil
}
