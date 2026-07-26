package main

import (
	"api-gateway/internal/config"
	"log/slog"
	"os"
)

func bootstrap() (config.Config, *slog.Logger, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	return cfg, logger, nil
}
