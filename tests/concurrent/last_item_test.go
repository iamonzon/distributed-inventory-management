package concurrent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"distributed-inventory-management/pkg/inventory"
	"distributed-inventory-management/pkg/models"
	"distributed-inventory-management/pkg/store"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastItemConcurrency(t *testing.T) {
	// Setup Service A
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Setup: Item with quantity = 1 (last item scenario)
	item := models.InventoryItem{
		ItemID:   "SKU-LAST",
		Name:     "Last Item",
		Quantity: 1,
		Version:  1,
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	// Start Service A server with proper router
	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA, serviceAAddr := startTestServer(routerA)
	defer serverA.Close()

	// Setup Service B
	cache := store.NewCache()

	// Initialize cache with the test item (simulate polling behavior)
	cache.Set("SKU-LAST", item)

	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Start Service B server with proper router
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

	// 10 stores try to buy the last item simultaneously
	numStores := 10
	var wg sync.WaitGroup
	results := make([]map[string]interface{}, numStores)

	start := time.Now()

	for i := 0; i < numStores; i++ {
		wg.Add(1)
		go func(storeID int) {
			defer wg.Done()

			req := map[string]interface{}{
				"item_id":  "SKU-LAST",
				"quantity": 1,
			}

			resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
			if err != nil {
				results[storeID] = map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}
			} else {
				results[storeID] = resp
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Count successes
	successCount := 0
	for i, result := range results {
		if success, ok := result["success"].(bool); ok && success {
			successCount++
		} else if err, ok := result["error"].(string); ok {
			t.Logf("Store %d error: %s", i, err)
		}
	}

	t.Logf("Concurrent checkout completed in %v", duration)
	t.Logf("Success count: %d/%d", successCount, numStores)

	// Verify: Exactly 1 success
	assert.Equal(t, 1, successCount, "Exactly one checkout must succeed")

	// Verify: Final quantity is 0
	finalItem, err := db.GetItem("SKU-LAST")
	require.NoError(t, err)
	assert.Equal(t, 0, finalItem.Quantity, "Final quantity must be 0")
}

func TestHighContentionCheckout(t *testing.T) {
	// Setup Service A
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Setup: Item with quantity = 50
	item := models.InventoryItem{
		ItemID:   "SKU-HIGH",
		Name:     "High Contention Item",
		Quantity: 50,
		Version:  1,
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	// Start Service A server with proper router
	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA, serviceAAddr := startTestServer(routerA)
	defer serverA.Close()

	// Setup Service B
	cache := store.NewCache()

	// Initialize cache with the test item (simulate polling behavior)
	cache.Set("SKU-HIGH", item)

	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	// Start Service B server with proper router
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

	// 50 concurrent checkouts for same item
	numCheckouts := 50
	var wg sync.WaitGroup
	results := make([]map[string]interface{}, numCheckouts)

	start := time.Now()

	for i := 0; i < numCheckouts; i++ {
		wg.Add(1)
		go func(checkoutID int) {
			defer wg.Done()

			req := map[string]interface{}{
				"item_id":  "SKU-HIGH",
				"quantity": 1,
			}

			resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
			if err != nil {
				results[checkoutID] = map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}
			} else {
				results[checkoutID] = resp
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Count successes
	successCount := 0
	for _, result := range results {
		if result["success"].(bool) {
			successCount++
		}
	}

	t.Logf("High contention checkout completed in %v", duration)
	t.Logf("Success count: %d/%d", successCount, numCheckouts)

	// Verify: Final quantity matches expected (initial - successes)
	finalItem, err := db.GetItem("SKU-HIGH")
	require.NoError(t, err)
	expectedFinal := 50 - successCount
	assert.Equal(t, expectedFinal, finalItem.Quantity,
		"Final quantity must equal initial (50) minus successful checkouts (%d)", successCount)

	// Verify: No overselling occurred
	assert.GreaterOrEqual(t, finalItem.Quantity, 0, "Must not oversell (quantity >= 0)")

	// Verify: At least some checkouts succeeded despite contention
	assert.Greater(t, successCount, 0, "At least one checkout should succeed")

	// Note: With 50 concurrent requests and max 5 retries, not all will succeed.
	// This is expected behavior - the system prevents overselling rather than
	// guaranteeing 100% success rate under extreme contention.
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
