package main

import (
	"auth-service/internal/infrastructure"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	grpctransport "auth-service/internal/transport/grpc"
	httptransport "auth-service/internal/transport/http"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	authv1 "auth-service/gen/auth/v1"
	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
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
	grpcHandler := grpctransport.NewServer(authService, menuService, jwtService, logger)
	healthServer := grpchealth.NewServer()
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpctransport.LoggingUnaryInterceptor(logger),
			grpctransport.RecoveryUnaryInterceptor(logger),
		),
	)
	authv1.RegisterAuthServiceServer(grpcServer, grpcHandler)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

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

	grpcListener, err := net.Listen("tcp", cfg.GRPC.Address())
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	return serve(server, grpcServer, grpcListener, logger)
}
