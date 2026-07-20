package main

import (
	"auth-service/internal/config"
	"auth-service/internal/infrastructure"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	httptransport "auth-service/internal/transport/http"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("auth-service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, logger, err := bootstrap()
	if err != nil {
		return err
	}

	ctx := context.Background()

	db, err := infrastructure.NewPostgres(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer db.Close()

	// redisClient, err := infrastructure.NewRedis(ctx, cfg)
	// if err != nil {
	// 	return fmt.Errorf("connect redis: %w", err)
	// }
	// defer closeWithLog(logger, "redis", redisClient.Close)

	// rabbitPublisher, err := infrastructure.NewRabbitMQPublisher(cfg)
	// if err != nil {
	// 	return fmt.Errorf("connect rabbitmq: %w", err)
	// }
	// defer closeWithLog(logger, "rabbitmq", rabbitPublisher.Close)

	userRepo := repository.NewPostgresUserRepository(db)
	jwtService := service.NewJWTService(cfg.JWT)
	authService := service.NewAuthService(cfg.Auth, userRepo, jwtService, nil, nil, logger)

	router := httptransport.NewRouter(cfg, logger, authService, jwtService)
	server := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return serve(server, logger)
}

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

func serve(server *http.Server, logger *slog.Logger) error {
	serverErr := make(chan error, 1)

	go func() {
		logger.Info("auth-service listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("listen and serve: %w", err)
			return
		}
		serverErr <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-serverErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	if err := <-serverErr; err != nil {
		return err
	}

	logger.Info("auth-service stopped gracefully")
	return nil
}

func closeWithLog(logger *slog.Logger, name string, closeFn func() error) {
	if err := closeFn(); err != nil {
		logger.Error("close resource", "resource", name, "error", err)
	}
}
