package chaos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"distributed-inventory-management/pkg/store"
	"distributed-inventory-management/tests/testhelpers"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceAFailure(t *testing.T) {
	// Setup services
	db, cache, serverA, _, _, serviceBAddr, cleanup := testhelpers.SetupTestServices(t)
	defer cleanup()

	// Seed test data
	testhelpers.SeedTestItem(t, db, "SKU-123", "Test Item", 10, 1)

	// Initialize cache (simulates successful polling)
	testhelpers.InitializeTestCache(t, cache, db)

	// Test: Normal checkout first (should succeed)
	req := map[string]any{
		"item_id":  "SKU-123",
		"quantity": 2,
	}

	resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool), "First checkout should succeed")

	// Simulate Service A failure by shutting it down
	serverA.Close()
	time.Sleep(100 * time.Millisecond)

	// Test: Checkout should fail gracefully when Service A is down
	req = map[string]any{
		"item_id":  "SKU-123",
		"quantity": 1,
	}

	resp, err = makeCheckoutRequest("http://"+serviceBAddr, req)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool), "Checkout should fail when Service A is down")

	// The error should indicate a connection problem or service unavailability
	errorMsg, hasError := resp["error"].(string)
	assert.True(t, hasError, "Response should contain an error message")
	// Accept either connection refused or other network errors
	assert.NotEmpty(t, errorMsg, "Error message should not be empty")
}

func TestNetworkDelay(t *testing.T) {
	// Setup Service A with artificial delay (within timeout, but noticeable)
	// CheckoutService client has 5 second timeout, so 2 second delay is acceptable
	db, cache, serverA, serverB, _, serviceBAddr, cleanup := testhelpers.SetupTestServicesWithDelay(
		t,
		2*time.Second, // Delay that's noticeable but within timeout
	)
	defer cleanup()

	// Seed test data
	testhelpers.SeedTestItem(t, db, "SKU-123", "Test Item", 10, 1)

	// Initialize cache (simulates successful polling)
	testhelpers.InitializeTestCache(t, cache, db)

	// Test: System should handle network delays gracefully and still succeed
	req := map[string]any{
		"item_id":  "SKU-123",
		"quantity": 1,
	}

	start := time.Now()
	resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
	duration := time.Since(start)

	require.NoError(t, err, "HTTP request should complete without error")
	assert.True(t, resp["success"].(bool), "Checkout should succeed despite delay")

	// Verify: Request should take at least as long as the delay
	assert.GreaterOrEqual(t, duration, 2*time.Second, "Should take at least 2 seconds due to delay")
	assert.Less(t, duration, 4*time.Second, "Should complete within reasonable time")

	// Verify final state
	finalItem, err := db.GetItem("SKU-123")
	require.NoError(t, err)
	assert.Equal(t, 9, finalItem.Quantity, "Quantity should be decremented")

	_ = serverA // Suppress unused warning
	_ = serverB
}

func TestServiceBCrashRecovery(t *testing.T) {
	// Setup services
	db, cache, serverA, serverB, serviceAAddr, serviceBAddr, cleanup := testhelpers.SetupTestServices(t)
	defer cleanup()

	// Seed test data
	testhelpers.SeedTestItem(t, db, "SKU-123", "Test Item", 10, 1)

	// Initialize cache (simulates successful polling)
	testhelpers.InitializeTestCache(t, cache, db)

	// Test: Normal operation first
	req := map[string]any{
		"item_id":  "SKU-123",
		"quantity": 2,
	}

	resp, err := makeCheckoutRequest("http://"+serviceBAddr, req)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool), "First checkout should succeed")

	// Simulate Service B crash by shutting it down
	serverB.Close()
	time.Sleep(100 * time.Millisecond)

	// Test: Service B should be unreachable
	_, err = makeCheckoutRequest("http://"+serviceBAddr, req)
	assert.Error(t, err, "Service B should be unreachable after crash")

	// Restart Service B (simulate recovery)
	cache2 := store.NewCache()
	checkoutSvc2 := store.NewCheckoutService(cache2, "http://"+serviceAAddr)
	storeSvc2 := store.NewStoreService(cache2, checkoutSvc2)

	// Initialize new cache with current database state
	testhelpers.InitializeTestCache(t, cache2, db)

	serverB, serviceBAddr = testhelpers.StartTestServer(setupStoreRoutes(storeSvc2))
	defer serverB.Close()

	// Test: Service B should work again after restart
	resp, err = makeCheckoutRequest("http://"+serviceBAddr, req)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool), "Checkout should succeed after Service B recovery")

	_ = serverA // Suppress unused warning
}

// Helper function to setup store routes
func setupStoreRoutes(storeSvc *store.StoreService) http.Handler {
	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)
	return router
}

func makeCheckoutRequest(serviceURL string, req map[string]any) (map[string]any, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(serviceURL+"/store/checkout", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}
