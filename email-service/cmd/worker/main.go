package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"email-service/internal/config"
	"email-service/internal/mailer"
	"email-service/internal/worker"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("email-service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	otpMailer := mailer.New(cfg.SMTP)
	consumer := worker.NewConsumer(cfg.RabbitMQ, otpMailer, logger)
	return consumer.Run(ctx)
}
