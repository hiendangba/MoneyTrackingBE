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
	menuRepo := repository.NewPostgresMenuRepository(db)
	jwtService, err := service.NewJWTService(cfg.JWT)
	if err != nil {
		return fmt.Errorf("create jwt service: %w", err)
	}
	authService := service.NewAuthService(cfg.Auth, userRepo, jwtService, redisClient, rabbitPublisher)
	menuService := service.NewMenuService(menuRepo)

	router, err := httptransport.NewRouter(cfg, logger, authService, menuService, jwtService, redisClient)
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}
	server := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	return serve(server, logger)
}
