package store

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"distributed-inventory-management/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckoutService_RetryLogic(t *testing.T) {
	// Track number of attempts
	attemptCount := 0

	// Create mock server that simulates version conflicts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++

		// Simulate version conflict on first attempt, success on second
		if attemptCount == 1 {
			// Version conflict response
			resp := models.CheckoutResponse{
				Success:         false,
				VersionConflict: true,
				CurrentVersion:  2,
				CurrentQuantity: 5,
				Message:         "Version conflict",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		} else {
			// Success response (must include current version/quantity per our fix)
			resp := models.CheckoutResponse{
				Success:         true,
				Message:         "Checkout successful",
				CurrentVersion:  2,
				CurrentQuantity: 3, // 5 - 2 = 3 after checking out 2 items
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	// Setup cache with initial item
	cache := NewCache()
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	cache.Set("SKU-123", item)

	// Create checkout service
	checkoutSvc := NewCheckoutService(cache, server.URL)

	// Test: Checkout with retry
	err := checkoutSvc.CheckoutWithRetry("SKU-123", 2)

	// Verify: Success after retry
	assert.NoError(t, err)

	// Verify: Cache was updated with new version and quantity after successful checkout
	updated, exists := cache.Get("SKU-123")
	require.True(t, exists)
	assert.Equal(t, 2, updated.Version)    // Updated to version 2 after retry
	assert.Equal(t, 3, updated.Quantity)   // 5 - 2 = 3 after successful checkout
}

func TestCheckoutService_MaxRetriesExceeded(t *testing.T) {
	// Create mock server that always returns version conflict
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.CheckoutResponse{
			Success:         false,
			VersionConflict: true,
			CurrentVersion:  2,
			CurrentQuantity: 5,
			Message:         "Version conflict",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Setup cache
	cache := NewCache()
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	cache.Set("SKU-123", item)

	// Create checkout service
	checkoutSvc := NewCheckoutService(cache, server.URL)

	// Test: Checkout with retry (should fail after max retries)
	err := checkoutSvc.CheckoutWithRetry("SKU-123", 2)

	// Verify: Max retries exceeded error
	assert.Equal(t, models.ErrMaxRetriesExceeded, err)
}

func TestCheckoutService_InsufficientStock(t *testing.T) {
	// Create mock server that returns insufficient stock
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.CheckoutResponse{
			Success:           false,
			InsufficientStock: true,
			CurrentVersion:    1,
			CurrentQuantity:   1,
			Message:           "Insufficient stock",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Setup cache
	cache := NewCache()
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	cache.Set("SKU-123", item)

	// Create checkout service
	checkoutSvc := NewCheckoutService(cache, server.URL)

	// Test: Checkout with insufficient stock
	err := checkoutSvc.CheckoutWithRetry("SKU-123", 5)

	// Verify: Out of stock error
	assert.Equal(t, models.ErrOutOfStock, err)
}

func TestCheckoutService_ItemNotFound(t *testing.T) {
	// Setup empty cache
	cache := NewCache()

	// Create checkout service
	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")

	// Test: Checkout non-existent item
	err := checkoutSvc.CheckoutWithRetry("NONEXISTENT", 1)

	// Verify: Item not found error
	assert.Equal(t, models.ErrItemNotFound, err)
}

func TestCheckoutService_ExponentialBackoff(t *testing.T) {
	// Create mock server that always returns version conflict
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.CheckoutResponse{
			Success:         false,
			VersionConflict: true,
			CurrentVersion:  2,
			CurrentQuantity: 5,
			Message:         "Version conflict",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Setup cache
	cache := NewCache()
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	cache.Set("SKU-123", item)

	// Create checkout service
	checkoutSvc := NewCheckoutService(cache, server.URL)

	// Test: Measure backoff timing
	start := time.Now()
	err := checkoutSvc.CheckoutWithRetry("SKU-123", 2)
	duration := time.Since(start)

	// Verify: Max retries exceeded
	assert.Equal(t, models.ErrMaxRetriesExceeded, err)

	// Verify: Total time includes backoff delays
	// With 5 attempts and exponential backoff, should take at least some time
	assert.Greater(t, duration, 10*time.Millisecond, "Backoff should introduce delays")
}

func TestCheckoutService_HTTPError(t *testing.T) {
	// Create mock server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Setup cache
	cache := NewCache()
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	cache.Set("SKU-123", item)

	// Create checkout service
	checkoutSvc := NewCheckoutService(cache, server.URL)

	// Test: Checkout with HTTP error
	err := checkoutSvc.CheckoutWithRetry("SKU-123", 2)

	// Verify: HTTP error is returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}
