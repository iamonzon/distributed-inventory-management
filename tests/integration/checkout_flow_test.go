package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"distributed-inventory-management/pkg/inventory"
	"distributed-inventory-management/pkg/models"
	"distributed-inventory-management/pkg/store"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEndCheckoutFlow(t *testing.T) {
	// Setup Service A
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Seed test data
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	// Start Service A server
	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA, serviceAAddr := startTestServer(routerA)
	defer serverA.Close()

	// Setup Service B
	cache := store.NewCache()

	// Initialize cache with the test item (simulate polling behavior)
	cache.Set("SKU-123", item)

	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Start Service B server
	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB, serviceBAddr := startTestServer(routerB)
	defer serverB.Close()

	// Wait for services to be ready - poll health endpoints
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + serviceAAddr + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + serviceBAddr + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Test: Normal checkout flow
	t.Run("NormalCheckout", func(t *testing.T) {
		// Verify initial state in Service A
		initialItem, err := db.GetItem("SKU-123")
		require.NoError(t, err)
		t.Logf("Initial DB state: quantity=%d, version=%d", initialItem.Quantity, initialItem.Version)

		// Verify cache in Service B
		cachedItem, exists := cache.Get("SKU-123")
		require.True(t, exists, "Item should be in cache")
		t.Logf("Initial cache state: quantity=%d, version=%d", cachedItem.Quantity, cachedItem.Version)

		req := map[string]interface{}{
			"item_id":  "SKU-123",
			"quantity": 3,
		}

		resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
		require.NoError(t, err)

		t.Logf("Response: %+v", resp)
		assert.True(t, resp["success"].(bool))
		assert.Equal(t, "Checkout successful", resp["message"])

		// Verify: Quantity reduced in Service A
		item, err := db.GetItem("SKU-123")
		require.NoError(t, err)
		assert.Equal(t, 7, item.Quantity)
		assert.Equal(t, 2, item.Version)
	})

	// Test: Insufficient stock
	t.Run("InsufficientStock", func(t *testing.T) {
		req := map[string]interface{}{
			"item_id":  "SKU-123",
			"quantity": 10, // More than available (7)
		}

		resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
		require.NoError(t, err)

		assert.False(t, resp["success"].(bool))
		assert.Equal(t, "Insufficient stock available", resp["message"])
	})

	// Test: Item not found
	t.Run("ItemNotFound", func(t *testing.T) {
		req := map[string]interface{}{
			"item_id":  "NONEXISTENT",
			"quantity": 1,
		}

		resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
		require.NoError(t, err)

		assert.False(t, resp["success"].(bool))
		assert.Equal(t, "Item not found", resp["message"])
	})
}

func TestVersionConflictResolution(t *testing.T) {
	// Setup Service A
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Seed test data
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	// Start Service A server
	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA, serviceAAddr := startTestServer(routerA)
	defer serverA.Close()

	// Setup Service B with stale cache
	cache := store.NewCache()
	// Simulate stale cache with old version
	staleItem := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  0, // Stale version
	}
	cache.Set("SKU-123", staleItem)

	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Start Service B server
	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB, serviceBAddr := startTestServer(routerB)
	defer serverB.Close()

	// Wait for services to be ready - poll health endpoints
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + serviceAAddr + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + serviceBAddr + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Test: Version conflict should be resolved with retry
	req := map[string]interface{}{
		"item_id":  "SKU-123",
		"quantity": 2,
	}

	resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
	require.NoError(t, err)

	assert.True(t, resp["success"].(bool))
	assert.Equal(t, "Checkout successful", resp["message"])

	// Verify: Cache was updated with correct version
	cached, exists := cache.Get("SKU-123")
	require.True(t, exists)
	assert.Equal(t, 2, cached.Version) // Should be updated to current version (1 -> 2 after successful checkout)
}

func makeCheckoutRequest(serviceURL string, req map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(serviceURL+"/store/checkout", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// startTestServer starts an HTTP server for testing with the provided handler on a dynamic port.
// Returns the server and the actual address it's listening on.
func startTestServer(handler http.Handler) (*http.Server, string) {
	// Listen on a random available port (port 0)
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(fmt.Sprintf("Failed to create listener: %v", err))
	}

	server := &http.Server{
		Handler: handler,
	}

	go func() {
		_ = server.Serve(listener) // Error expected when server is closed
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	return server, listener.Addr().String()
}
