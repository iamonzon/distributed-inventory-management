package testhelpers

import (
	"io"
	"log/slog"
	"os"
)

func init() {
	SetupTestLogging()
}

// SetupTestLogging configures logging for tests with environment variable control.
//
// By default, tests run with ERROR level logging to keep output clean.
// Use environment variables to control behavior:
//
//   - TEST_LOG_LEVEL=debug  - Show all logs (DEBUG, INFO, WARN, ERROR)
//   - TEST_LOG_LEVEL=info   - Show INFO, WARN, ERROR
//   - TEST_LOG_LEVEL=warn   - Show WARN, ERROR
//   - TEST_LOG_LEVEL=error  - Show ERROR only (default)
//   - TEST_LOG_SILENT=true  - Discard all logs completely
//
// Examples:
//
//	# Normal test run (clean output)
//	go test ./...
//
//	# Debug a failing test (verbose output)
//	TEST_LOG_LEVEL=debug go test ./pkg/store -run TestCheckout -v
//
//	# Completely silent (for benchmarks)
//	TEST_LOG_SILENT=true go test -bench .
func SetupTestLogging() {
	// Default to ERROR level for clean test output
	level := slog.LevelError

	// Allow override via environment variable
	switch os.Getenv("TEST_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	// Allow complete silence (useful for benchmarks)
	var output io.Writer = os.Stdout
	if os.Getenv("TEST_LOG_SILENT") == "true" {
		output = io.Discard
	}

	// Create and set the logger
	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}
