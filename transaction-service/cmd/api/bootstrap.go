package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/config"
)

func bootstrap() (config.Config, *slog.Logger, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)
	return cfg, logger, nil
}
