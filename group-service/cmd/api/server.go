package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
)

func serve(server *grpc.Server, listener net.Listener, logger *slog.Logger, stop func()) error {
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

	receivedSignal := false
	select {
	case sig := <-stopSignal:
		logger.Info("shutdown signal received", "signal", sig.String())
		receivedSignal = true
	case err := <-serverErr:
		if err != nil {
			return err
		}
		return nil
	}

	if receivedSignal {
		shutdownDone := make(chan struct{})
		go func() {
			stop()
			close(shutdownDone)
		}()

		select {
		case <-shutdownDone:
		case <-time.After(15 * time.Second):
			server.Stop()
		}

		if err := <-serverErr; err != nil {
			return err
		}
		logger.Info("group-service stopped gracefully")
		return nil
	}
	return nil
}
