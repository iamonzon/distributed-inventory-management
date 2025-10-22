package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"distributed-inventory-management/pkg/store"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Parse command line flags
	var (
		port     = flag.String("port", "8081", "Port to listen on")
		serviceA = flag.String("service-a", "http://localhost:8080", "Service A URL")
		interval = flag.Duration("interval", 30*time.Second, "Polling interval")
	)
	flag.Parse()

	// Initialize cache and services
	cache := store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, *serviceA)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Create and start poller
	poller := store.NewPoller(cache, *serviceA, *interval)
	go poller.StartPolling()
	defer poller.Stop()

	// Setup routes
	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + *port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("service starting",
			"service", "store-service-b",
			"port", *port,
			"service_a", *serviceA,
			"poll_interval", *interval)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down service", "service", "service-b")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("service stopped", "service", "service-b")
}
