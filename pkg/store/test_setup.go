package store

import (
	"io"
	"log/slog"
	"os"
)

func init() {
	// Configure logging for tests: ERROR level by default, configurable via env vars
	setupTestLogging()
}

func setupTestLogging() {
	level := slog.LevelError // Default: only errors

	switch os.Getenv("TEST_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	}

	var output io.Writer = os.Stdout
	if os.Getenv("TEST_LOG_SILENT") == "true" {
		output = io.Discard
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}
