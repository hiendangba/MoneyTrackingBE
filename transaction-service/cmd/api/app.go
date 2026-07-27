package main

import (
	"context"
	"fmt"
	"net"
	"time"

	transactionv1 "transaction-service/gen/transaction/v1"
	"transaction-service/internal/clients"
	"transaction-service/internal/infrastructure"
	"transaction-service/internal/repository"
	"transaction-service/internal/service"
	grpctransport "transaction-service/internal/transport/grpc"

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

	directories, err := clients.New(
		ctx, cfg.AuthServiceAddress, cfg.GroupServiceAddress, cfg.UpstreamTimeout,
	)
	if err != nil {
		return fmt.Errorf("create upstream clients: %w", err)
	}
	defer func() {
		if err := directories.Close(); err != nil {
			logger.Error("close upstream clients", "error", err)
		}
	}()

	repo := repository.NewPostgresRepository(db)
	transactionService := service.NewTransactionService(
		repo, directories.Auth, directories.Group, cfg.DefaultCurrency,
	)
	handler := grpctransport.NewServer(transactionService, logger)
	healthServer := grpchealth.NewServer()
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpctransport.RecoveryUnaryInterceptor(logger),
			grpctransport.LoggingUnaryInterceptor(logger),
		),
	)
	transactionv1.RegisterTransactionServiceServer(server, handler)
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	listener, err := net.Listen("tcp", cfg.Server.Address())
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}
	logger.Info("transaction-service listening", "addr", cfg.Server.Address())
	return serve(server, listener, logger)
}

func stopServer(server *grpc.Server) {
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		server.Stop()
	}
}
