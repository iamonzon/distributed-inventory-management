package inventory

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"distributed-inventory-management/pkg/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetItemHandler_Success(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Seed test item
	item := models.InventoryItem{
		ItemID:   "TEST-001",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	// Setup service and handler
	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	// Make request
	req := httptest.NewRequest("GET", "/api/v1/inventory/TEST-001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var result models.InventoryItem
	err = json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "TEST-001", result.ItemID)
	assert.Equal(t, "Test Item", result.Name)
	assert.Equal(t, 10, result.Quantity)
}

func TestGetItemHandler_NotFound(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/inventory/NONEXISTENT", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Item not found")
}

func TestGetAllItemsHandler_Success(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Seed multiple items
	items := []models.InventoryItem{
		{ItemID: "SKU-001", Name: "Item 1", Quantity: 5, Version: 1},
		{ItemID: "SKU-002", Name: "Item 2", Quantity: 10, Version: 1},
		{ItemID: "SKU-003", Name: "Item 3", Quantity: 15, Version: 1},
	}
	for _, item := range items {
		err = db.SetItem(item)
		require.NoError(t, err)
	}

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/inventory/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var snapshot models.InventorySnapshot
	err = json.NewDecoder(w.Body).Decode(&snapshot)
	require.NoError(t, err)
	assert.Len(t, snapshot.Items, 3)
}

func TestGetAllItemsHandler_Empty(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/inventory/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var snapshot models.InventorySnapshot
	err = json.NewDecoder(w.Body).Decode(&snapshot)
	require.NoError(t, err)
	assert.Len(t, snapshot.Items, 0)
}

func TestCheckoutHandler_ValidationErrors(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	tests := []struct {
		name           string
		payload        interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid JSON",
			payload:        "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON",
		},
		{
			name: "Missing item_id",
			payload: models.CheckoutRequest{
				ItemID:          "",
				Quantity:        1,
				ExpectedVersion: 1,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "item_id is required",
		},
		{
			name: "Negative quantity",
			payload: models.CheckoutRequest{
				ItemID:          "SKU-001",
				Quantity:        -1,
				ExpectedVersion: 1,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "quantity must be positive",
		},
		{
			name: "Zero quantity",
			payload: models.CheckoutRequest{
				ItemID:          "SKU-001",
				Quantity:        0,
				ExpectedVersion: 1,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "quantity must be positive",
		},
		{
			name: "Negative expected_version",
			payload: models.CheckoutRequest{
				ItemID:          "SKU-001",
				Quantity:        1,
				ExpectedVersion: -1,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "expected_version must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if str, ok := tt.payload.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.payload)
				require.NoError(t, err)
			}

			req := httptest.NewRequest("POST", "/api/v1/checkout", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedError)
		})
	}
}

func TestCheckoutHandler_SuccessfulCheckout(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Seed item
	item := models.InventoryItem{
		ItemID:   "SKU-CHECKOUT",
		Name:     "Checkout Test",
		Quantity: 10,
		Version:  1,
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	// Valid checkout request
	checkoutReq := models.CheckoutRequest{
		ItemID:          "SKU-CHECKOUT",
		Quantity:        3,
		ExpectedVersion: 1,
	}
	body, err := json.Marshal(checkoutReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/checkout", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckoutResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Checkout successful", resp.Message)
	assert.Equal(t, 7, resp.CurrentQuantity) // 10 - 3
	assert.Equal(t, 2, resp.CurrentVersion)  // version incremented
}

func TestCheckoutHandler_VersionConflict(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Seed item
	item := models.InventoryItem{
		ItemID:   "SKU-VERSION",
		Name:     "Version Test",
		Quantity: 10,
		Version:  5, // Current version is 5
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	// Checkout with old version
	checkoutReq := models.CheckoutRequest{
		ItemID:          "SKU-VERSION",
		Quantity:        2,
		ExpectedVersion: 3, // Stale version
	}
	body, err := json.Marshal(checkoutReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/checkout", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckoutResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.True(t, resp.VersionConflict)
	assert.Equal(t, "Version conflict - item was modified by another operation", resp.Message)
	assert.Equal(t, 5, resp.CurrentVersion) // Returns current version
	assert.Equal(t, 10, resp.CurrentQuantity)
}

func TestCheckoutHandler_InsufficientStock(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Seed item with low quantity
	item := models.InventoryItem{
		ItemID:   "SKU-LOW",
		Name:     "Low Stock",
		Quantity: 2,
		Version:  1,
	}
	err = db.SetItem(item)
	require.NoError(t, err)

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	// Try to checkout more than available
	checkoutReq := models.CheckoutRequest{
		ItemID:          "SKU-LOW",
		Quantity:        5,
		ExpectedVersion: 1,
	}
	body, err := json.Marshal(checkoutReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/checkout", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckoutResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.True(t, resp.InsufficientStock)
	assert.Equal(t, "Insufficient stock available", resp.Message)
	assert.Equal(t, 2, resp.CurrentQuantity)
}

func TestHealthHandler(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]string
	err = json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "healthy", result["status"])
	assert.Equal(t, "inventory-service-a", result["service"])
}

func TestCreateOrUpdateItemHandler_Success(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	newItem := models.InventoryItem{
		ItemID:   "SKU-NEW",
		Name:     "New Item",
		Quantity: 20,
		Version:  1,
	}
	body, err := json.Marshal(newItem)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/admin/inventory", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, "Item created/updated successfully", resp["message"])

	// Verify item was created
	item, err := db.GetItem("SKU-NEW")
	require.NoError(t, err)
	assert.Equal(t, "New Item", item.Name)
	assert.Equal(t, 20, item.Quantity)
}

func TestCreateOrUpdateItemHandler_ValidationErrors(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	tests := []struct {
		name           string
		payload        interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid JSON",
			payload:        "not json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON",
		},
		{
			name: "Missing item_id",
			payload: models.InventoryItem{
				ItemID:   "",
				Name:     "Item",
				Quantity: 10,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "item_id is required",
		},
		{
			name: "Missing name",
			payload: models.InventoryItem{
				ItemID:   "SKU-001",
				Name:     "",
				Quantity: 10,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "name is required",
		},
		{
			name: "Negative quantity",
			payload: models.InventoryItem{
				ItemID:   "SKU-001",
				Name:     "Item",
				Quantity: -5,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "quantity must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if str, ok := tt.payload.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.payload)
				require.NoError(t, err)
			}

			req := httptest.NewRequest("POST", "/api/v1/admin/inventory", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedError)
		})
	}
}

func TestCreateOrUpdateItemHandler_DefaultVersion(t *testing.T) {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db)
	router := mux.NewRouter()
	service.SetupRoutes(router)

	// Create item without version (should default to 1)
	newItem := models.InventoryItem{
		ItemID:   "SKU-DEFAULT",
		Name:     "Default Version",
		Quantity: 5,
		Version:  0, // Should be normalized to 1
	}
	body, err := json.Marshal(newItem)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/admin/inventory", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify version was set to 1
	item, err := db.GetItem("SKU-DEFAULT")
	require.NoError(t, err)
	assert.Equal(t, 1, item.Version)
}
