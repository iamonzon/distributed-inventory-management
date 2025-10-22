package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"distributed-inventory-management/pkg/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetItemHandler_Success(t *testing.T) {
	cache := NewCache()
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	cache.Set("SKU-123", item)

	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/store/inventory/SKU-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result models.InventoryItem
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "SKU-123", result.ItemID)
	assert.Equal(t, 10, result.Quantity)
}

func TestGetItemHandler_NotFound(t *testing.T) {
	cache := NewCache()
	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/store/inventory/NONEXISTENT", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Item not found in cache")
}

func TestGetAllItemsHandler_Success(t *testing.T) {
	cache := NewCache()

	items := []models.InventoryItem{
		{ItemID: "SKU-001", Name: "Item 1", Quantity: 5, Version: 1},
		{ItemID: "SKU-002", Name: "Item 2", Quantity: 10, Version: 1},
		{ItemID: "SKU-003", Name: "Item 3", Quantity: 15, Version: 1},
	}

	for _, item := range items {
		cache.Set(item.ItemID, item)
	}

	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/store/inventory/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var snapshot models.InventorySnapshot
	err := json.NewDecoder(w.Body).Decode(&snapshot)
	require.NoError(t, err)
	assert.Len(t, snapshot.Items, 3)
}

func TestGetAllItemsHandler_Empty(t *testing.T) {
	cache := NewCache()
	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/store/inventory/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var snapshot models.InventorySnapshot
	err := json.NewDecoder(w.Body).Decode(&snapshot)
	require.NoError(t, err)
	assert.Len(t, snapshot.Items, 0)
}

func TestCheckoutHandler_ValidationErrors(t *testing.T) {
	cache := NewCache()
	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid JSON",
			payload:        "not valid json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON",
		},
		{
			name:           "Missing item_id",
			payload:        `{"quantity": 5}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "item_id is required",
		},
		{
			name:           "Empty item_id",
			payload:        `{"item_id": "", "quantity": 5}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "item_id is required",
		},
		{
			name:           "Zero quantity",
			payload:        `{"item_id": "SKU-123", "quantity": 0}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "quantity must be positive",
		},
		{
			name:           "Negative quantity",
			payload:        `{"item_id": "SKU-123", "quantity": -5}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "quantity must be positive",
		},
		{
			name:           "Missing quantity",
			payload:        `{"item_id": "SKU-123"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "quantity must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/store/checkout", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedError)
		})
	}
}

func TestCheckoutHandler_ItemNotFound(t *testing.T) {
	cache := NewCache()
	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	payload := `{"item_id": "NONEXISTENT", "quantity": 1}`
	req := httptest.NewRequest("POST", "/store/checkout", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "Item not found", resp["message"])
}

func TestCheckoutHandler_OutOfStock(t *testing.T) {
	// Setup mock server that returns out of stock
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.CheckoutResponse{
			Success:           false,
			InsufficientStock: true,
			CurrentQuantity:   0,
			CurrentVersion:    1,
			Message:           "Insufficient stock",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cache := NewCache()
	cache.Set("SKU-LOW", models.InventoryItem{
		ItemID:   "SKU-LOW",
		Name:     "Low Stock",
		Quantity: 0,
		Version:  1,
	})

	checkoutSvc := NewCheckoutService(cache, mockServer.URL)
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	payload := `{"item_id": "SKU-LOW", "quantity": 1}`
	req := httptest.NewRequest("POST", "/store/checkout", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "Insufficient stock available", resp["message"])
}

func TestCheckoutHandler_MaxRetriesExceeded(t *testing.T) {
	// Setup mock server that always returns version conflict
	attemptCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		resp := models.CheckoutResponse{
			Success:         false,
			VersionConflict: true,
			CurrentQuantity: 10,
			CurrentVersion:  attemptCount + 1, // Always ahead
			Message:         "Version conflict",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cache := NewCache()
	cache.Set("SKU-CONFLICT", models.InventoryItem{
		ItemID:   "SKU-CONFLICT",
		Name:     "Conflict Item",
		Quantity: 10,
		Version:  1,
	})

	checkoutSvc := NewCheckoutService(cache, mockServer.URL)
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	payload := `{"item_id": "SKU-CONFLICT", "quantity": 1}`
	req := httptest.NewRequest("POST", "/store/checkout", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "Checkout failed after maximum retries", resp["message"])

	// Verify it attempted max retries
	assert.Equal(t, 5, attemptCount)
}

func TestCheckoutHandler_Success(t *testing.T) {
	// Setup mock server that returns success
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.CheckoutResponse{
			Success:         true,
			CurrentQuantity: 7,
			CurrentVersion:  2,
			Message:         "Checkout successful",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cache := NewCache()
	cache.Set("SKU-SUCCESS", models.InventoryItem{
		ItemID:   "SKU-SUCCESS",
		Name:     "Success Item",
		Quantity: 10,
		Version:  1,
	})

	checkoutSvc := NewCheckoutService(cache, mockServer.URL)
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	payload := `{"item_id": "SKU-SUCCESS", "quantity": 3}`
	req := httptest.NewRequest("POST", "/store/checkout", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, "Checkout successful", resp["message"])

	// Verify cache was updated
	item, exists := cache.Get("SKU-SUCCESS")
	require.True(t, exists)
	assert.Equal(t, 7, item.Quantity)
	assert.Equal(t, 2, item.Version)
}

func TestCheckoutHandler_ServiceUnavailable(t *testing.T) {
	// Use a URL that will fail to connect
	cache := NewCache()
	cache.Set("SKU-TEST", models.InventoryItem{
		ItemID:   "SKU-TEST",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	})

	// Point to a port that's not listening
	checkoutSvc := NewCheckoutService(cache, "http://localhost:65535")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	payload := `{"item_id": "SKU-TEST", "quantity": 1}`
	req := httptest.NewRequest("POST", "/store/checkout", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Connection failures fall through to default case (500 Internal Server Error)
	// unless they exhaust retries (which returns ErrMaxRetriesExceeded -> 503)
	// In this case, the error type doesn't match the specific cases, so it's 500
	assert.Contains(t, []int{http.StatusInternalServerError, http.StatusServiceUnavailable}, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHealthHandler(t *testing.T) {
	cache := NewCache()

	// Add some items to cache
	for i := 1; i <= 5; i++ {
		cache.Set(fmt.Sprintf("SKU-%d", i), models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: i * 10,
			Version:  1,
		})
	}

	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "healthy", result["status"])
	assert.Equal(t, "store-service-b", result["service"])
	assert.Equal(t, float64(5), result["cache_size"]) // JSON numbers are float64
}

func TestHealthHandler_EmptyCache(t *testing.T) {
	cache := NewCache()
	checkoutSvc := NewCheckoutService(cache, "http://localhost:8080")
	storeSvc := NewStoreService(cache, checkoutSvc)

	router := mux.NewRouter()
	storeSvc.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, float64(0), result["cache_size"])
}
