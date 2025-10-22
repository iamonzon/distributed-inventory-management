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
	initializeLogger()
	config := parseCommandLineFlags()

	cache, storeSvc := initializeStoreServices(config.serviceA)
	poller := startBackgroundPoller(cache, config.serviceA, config.interval)
	defer poller.Stop()

	router := setupStoreRoutes(storeSvc)
	server := createHTTPServer(":"+config.port, router)

	startServerAsyncWithConfig(server, config)
	waitForShutdownSignal()

	shutdownGracefully(server, "service-b")
}

type serviceConfig struct {
	port     string
	serviceA string
	interval time.Duration
}

func initializeLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func parseCommandLineFlags() serviceConfig {
	port := flag.String("port", "8081", "Port to listen on")
	serviceA := flag.String("service-a", "http://localhost:8080", "Service A URL")
	interval := flag.Duration("interval", 30*time.Second, "Polling interval")
	flag.Parse()

	return serviceConfig{
		port:     *port,
		serviceA: *serviceA,
		interval: *interval,
	}
}

func initializeStoreServices(serviceAURL string) (*store.Cache, *store.StoreService) {
	cache := store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, serviceAURL)
	storeSvc := store.NewStoreService(cache, checkoutSvc)
	return cache, storeSvc
}

func startBackgroundPoller(cache *store.Cache, serviceAURL string, interval time.Duration) *store.Poller {
	poller := store.NewPoller(cache, serviceAURL, interval)
	go poller.StartPolling()
	return poller
}

func setupStoreRoutes(storeSvc *store.StoreService) *mux.Router {
	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)
	return router
}

func createHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}

func startServerAsyncWithConfig(server *http.Server, config serviceConfig) {
	go func() {
		slog.Info("service starting",
			"service", "store-service-b",
			"port", config.port,
			"service_a", config.serviceA,
			"poll_interval", config.interval)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()
}

func waitForShutdownSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func shutdownGracefully(server *http.Server, serviceName string) {
	slog.Info("shutting down service", "service", serviceName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("service stopped", "service", serviceName)
}
