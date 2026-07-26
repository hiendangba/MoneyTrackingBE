package main

import (
	"log/slog"
	"os"
)

func main() {
	if err := run(); err != nil {
		slog.Error("group-service stopped", "error", err)
		os.Exit(1)
	}
}

