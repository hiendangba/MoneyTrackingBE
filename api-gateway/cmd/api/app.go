package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"api-gateway/internal/clients"
	httptransport "api-gateway/internal/transport/http"
)

func run() error {
	cfg, logger, err := bootstrap()
	if err != nil {
		return err
	}

	ctx := context.Background()
	clients, err := clients.New(
		ctx,
		cfg.AuthServiceAddress,
		cfg.GroupServiceAddress,
		cfg.TransactionServiceAddress,
	)
	if err != nil {
		return fmt.Errorf("create clients: %w", err)
	}
	defer closeWithLog(logger, "grpc clients", clients.Close)

	gateway, err := httptransport.NewGateway(cfg, clients, logger)
	if err != nil {
		return fmt.Errorf("create gateway: %w", err)
	}

	server := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           gateway.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	return serve(server, logger)
}
