package store

import (
	"encoding/json"
	"net/http"

	"distributed-inventory-management/pkg/models"

	"github.com/gorilla/mux"
)

// StoreService represents the store service with cache and checkout capabilities
type StoreService struct {
	cache       *Cache
	checkoutSvc *CheckoutService
}

// NewStoreService creates a new store service
func NewStoreService(cache *Cache, checkoutSvc *CheckoutService) *StoreService {
	return &StoreService{
		cache:       cache,
		checkoutSvc: checkoutSvc,
	}
}

// GetItemHandler handles GET /store/inventory/:id
func (s *StoreService) GetItemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["id"]

	item, exists := s.cache.Get(itemID)
	if !exists {
		http.Error(w, "Item not found in cache", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(item); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetAllItemsHandler handles GET /store/inventory/all
func (s *StoreService) GetAllItemsHandler(w http.ResponseWriter, r *http.Request) {
	items := s.cache.GetAll()

	snapshot := models.InventorySnapshot{Items: items}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// CheckoutHandler handles POST /store/checkout
func (s *StoreService) CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ItemID   string `json:"item_id"`
		Quantity int    `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.ItemID == "" {
		http.Error(w, "item_id is required", http.StatusBadRequest)
		return
	}
	if req.Quantity <= 0 {
		http.Error(w, "quantity must be positive", http.StatusBadRequest)
		return
	}

	// Perform checkout with retry
	err := s.checkoutSvc.CheckoutWithRetry(req.ItemID, req.Quantity)

	// Build response
	resp := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Error   string `json:"error,omitempty"`
	}{}

	// Set Content-Type header before writing status
	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		resp.Success = false
		resp.Error = err.Error()

		switch err {
		case models.ErrOutOfStock:
			resp.Message = "Insufficient stock available"
			w.WriteHeader(http.StatusConflict)
		case models.ErrItemNotFound:
			resp.Message = "Item not found"
			w.WriteHeader(http.StatusNotFound)
		case models.ErrMaxRetriesExceeded:
			resp.Message = "Checkout failed after maximum retries"
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			resp.Message = "Checkout failed"
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		resp.Success = true
		resp.Message = "Checkout successful"
		w.WriteHeader(http.StatusOK)
	}

	// Encode response after status has been set
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HealthHandler handles GET /health
func (s *StoreService) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":     "healthy",
		"service":    "store-service-b",
		"cache_size": s.cache.Size(),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// SetupRoutes configures all routes for the store service
func (s *StoreService) SetupRoutes(r *mux.Router) {
	store := r.PathPrefix("/store").Subrouter()
	store.HandleFunc("/inventory/all", s.GetAllItemsHandler).Methods("GET")
	store.HandleFunc("/inventory/{id}", s.GetItemHandler).Methods("GET")
	store.HandleFunc("/checkout", s.CheckoutHandler).Methods("POST")
	r.HandleFunc("/health", s.HealthHandler).Methods("GET")
}
