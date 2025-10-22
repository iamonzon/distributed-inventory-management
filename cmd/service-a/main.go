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
	// Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Initialize database
	db, err := inventory.NewDatabase(":memory:") // Use in-memory SQLite for prototype
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Seed some test data
	seedTestData(db)

	// Create service and setup routes
	service := inventory.NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	// Start HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("service starting",
			"service", "inventory-service-a",
			"port", "8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down service", "service", "service-a")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("service stopped", "service", "service-a")
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
