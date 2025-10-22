package chaos

import (
	"bytes"
	"encoding/json"
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

func TestServiceAFailure(t *testing.T) {
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
	serverA := startTestServer(":8080", routerA)
	defer serverA.Close()

	// Setup Service B
	cache := store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Start Service B server
	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB := startTestServer(":8081", routerB)
	defer serverB.Close()

	// Wait for services to be ready
	time.Sleep(100 * time.Millisecond)

	// Test: Normal checkout first
	req := map[string]interface{}{
		"item_id":  "SKU-123",
		"quantity": 2,
	}

	resp, err := makeCheckoutRequest("http://localhost:8081", req)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	// Simulate Service A failure by shutting it down
	serverA.Close()
	time.Sleep(100 * time.Millisecond)

	// Test: Checkout should fail gracefully
	req = map[string]interface{}{
		"item_id":  "SKU-123",
		"quantity": 1,
	}

	resp, err = makeCheckoutRequest("http://localhost:8081", req)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
	assert.Contains(t, resp["error"], "status 500")
}

func TestNetworkDelay(t *testing.T) {
	// Setup Service A with artificial delay
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

	// Start Service A server with delay middleware
	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA := startTestServerWithDelay(":8080", routerA, 2*time.Second)
	defer serverA.Close()

	// Setup Service B with shorter timeout
	cache := store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Start Service B server
	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB := startTestServer(":8081", routerB)
	defer serverB.Close()

	// Wait for services to be ready
	time.Sleep(100 * time.Millisecond)

	// Test: Checkout should timeout due to network delay
	req := map[string]interface{}{
		"item_id":  "SKU-123",
		"quantity": 1,
	}

	start := time.Now()
	resp, err := makeCheckoutRequest("http://localhost:8081", req)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
	assert.Contains(t, resp["error"], "timeout")

	// Verify: Timeout occurred within expected range
	assert.Greater(t, duration, 1*time.Second, "Should take at least 1 second")
	assert.Less(t, duration, 3*time.Second, "Should timeout before 3 seconds")
}

func TestServiceBCrashRecovery(t *testing.T) {
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
	serverA := startTestServer(":8080", routerA)
	defer serverA.Close()

	// Setup Service B
	cache := store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Start Service B server
	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB := startTestServer(":8081", routerB)
	defer serverB.Close()

	// Wait for services to be ready
	time.Sleep(100 * time.Millisecond)

	// Test: Normal operation first
	req := map[string]interface{}{
		"item_id":  "SKU-123",
		"quantity": 2,
	}

	resp, err := makeCheckoutRequest("http://localhost:8081", req)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	// Simulate Service B crash by shutting it down
	serverB.Close()
	time.Sleep(100 * time.Millisecond)

	// Test: Service B should be unreachable
	_, err = makeCheckoutRequest("http://localhost:8081", req)
	assert.Error(t, err, "Service B should be unreachable")

	// Restart Service B (simulate recovery)
	routerB2 := mux.NewRouter()
	storeSvc.SetupRoutes(routerB2)
	serverB = startTestServer(":8081", routerB2)
	defer serverB.Close()
	time.Sleep(100 * time.Millisecond)

	// Test: Service B should work again after restart
	resp, err = makeCheckoutRequest("http://localhost:8081", req)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
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

func startTestServer(addr string, handler http.Handler) *http.Server {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()

	return server
}

func startTestServerWithDelay(addr string, handler http.Handler, delay time.Duration) *http.Server {
	// Wrap handler with delay middleware
	delayedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		handler.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: delayedHandler,
	}

	go func() {
		server.ListenAndServe()
	}()

	return server
}
