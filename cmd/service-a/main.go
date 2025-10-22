package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"distributed-inventory-management/pkg/inventory"
	"distributed-inventory-management/pkg/models"

	"github.com/gorilla/mux"
)

func main() {
	initializeLogger()
	db := initializeDatabase()
	defer db.Close()

	seedTestData(db)

	router := setupServiceAndRoutes(db)
	server := createHTTPServer(":8080", router)

	startServerAsync(server, "inventory-service-a", "8080")
	waitForShutdownSignal()

	shutdownGracefully(server, "service-a")
}

func initializeLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func initializeDatabase() *inventory.Database {
	db, err := inventory.NewDatabase(":memory:")
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	return db
}

func setupServiceAndRoutes(db *inventory.Database) *mux.Router {
	service := inventory.NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)
	return router
}

func createHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}

func startServerAsync(server *http.Server, serviceName, port string) {
	go func() {
		slog.Info("service starting", "service", serviceName, "port", port)
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

// seedTestData populates the database with sample inventory items
func seedTestData(db *inventory.Database) {
	items := []struct {
		itemID   string
		name     string
		quantity int
		version  int
	}{
		{"SKU-123", "Wireless Headphones", 50, 1},
		{"SKU-456", "Smartphone Case", 100, 1},
		{"SKU-789", "USB-C Cable", 200, 1},
		{"SKU-101", "Bluetooth Speaker", 25, 1},
		{"SKU-202", "Laptop Stand", 15, 1},
	}

	for _, item := range items {
		if err := db.SetItem(models.InventoryItem{
			ItemID:   item.itemID,
			Name:     item.name,
			Quantity: item.quantity,
			Version:  item.version,
		}); err != nil {
			slog.Error("failed to seed item",
				"item_id", item.itemID,
				"error", err)
			os.Exit(1)
		}
	}

	slog.Info("database seeded",
		"item_count", len(items))
}
