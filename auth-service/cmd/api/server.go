package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
)

func serve(httpServer *http.Server, grpcServer *grpc.Server, grpcListener net.Listener, logger *slog.Logger) error {
	httpErr := make(chan error, 1)
	grpcErr := make(chan error, 1)

	go func() {
		logger.Info("auth-service http listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			httpErr <- fmt.Errorf("listen and serve http: %w", err)
			return
		}
		httpErr <- nil
	}()

	go func() {
		logger.Info("auth-service grpc listening", "addr", grpcListener.Addr().String())
		if err := grpcServer.Serve(grpcListener); err != nil {
			grpcErr <- fmt.Errorf("serve grpc: %w", err)
			return
		}
		grpcErr <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-httpErr:
		if err != nil {
			return err
		}
	case err := <-grpcErr:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful http shutdown: %w", err)
	}

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
	case <-time.After(10 * time.Second):
		grpcServer.Stop()
	}

	if err := <-httpErr; err != nil {
		return err
	}
	if err := <-grpcErr; err != nil {
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
