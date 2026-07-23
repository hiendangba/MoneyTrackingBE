package main

import (
	"auth-service/internal/infrastructure"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	httptransport "auth-service/internal/transport/http"
	"context"
	"fmt"
	"net/http"
	"time"
)

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

	redisClient, err := infrastructure.NewRedis(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer closeWithLog(logger, "redis", redisClient.Close)

	rabbitPublisher, err := infrastructure.NewRabbitMQPublisher(cfg)
	if err != nil {
		return fmt.Errorf("connect rabbitmq: %w", err)
	}
	defer closeWithLog(logger, "rabbitmq", rabbitPublisher.Close)

	userRepo := repository.NewPostgresUserRepository(db)
	jwtService := service.NewJWTService(cfg.JWT)
	authService := service.NewAuthService(cfg.Auth, userRepo, jwtService, redisClient, rabbitPublisher, logger)

	router := httptransport.NewRouter(cfg, logger, authService, jwtService)
	server := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return serve(server, logger)
}
