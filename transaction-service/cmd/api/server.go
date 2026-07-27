package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
)

func serve(server *grpc.Server, listener net.Listener, logger *slog.Logger) error {
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil {
			serverErr <- fmt.Errorf("serve grpc: %w", err)
			return
		}
		serverErr <- nil
	}()

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stopSignal)

	select {
	case signalValue := <-stopSignal:
		logger.Info("shutdown signal received", "signal", signalValue.String())
		stopServer(server)
		if err := <-serverErr; err != nil {
			return err
		}
		logger.Info("transaction-service stopped gracefully")
		return nil
	case err := <-serverErr:
		return err
	}
}
