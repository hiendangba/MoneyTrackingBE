package main

import (
	"context"
	"fmt"
	"net"
	"time"

	groupv1 "group-service/gen/group/v1"
	"group-service/internal/infrastructure"
	"group-service/internal/repository"
	"group-service/internal/service"
	grpctransport "group-service/internal/transport/grpc"

	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc"
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

	groupRepo := repository.NewPostgresGroupRepository(db)
	groupService := service.NewGroupService(db, groupRepo)
	grpcServer := grpctransport.NewServer(groupService, logger)
	healthServer := grpchealth.NewServer()

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpctransport.RecoveryUnaryInterceptor(logger),
			grpctransport.LoggingUnaryInterceptor(logger),
		),
	)
	groupv1.RegisterGroupServiceServer(server, grpcServer)
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	listener, err := net.Listen("tcp", cfg.Server.Address())
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}
	logger.Info("group-service listening", "addr", cfg.Server.Address())

	stop := func() {
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

	return serve(server, listener, logger, stop)
}
