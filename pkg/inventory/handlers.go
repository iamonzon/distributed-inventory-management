package inventory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"distributed-inventory-management/pkg/models"

	"github.com/gorilla/mux"
)

// Service represents the central inventory service
type Service struct {
	db *Database
}

// NewService creates a new inventory service
func NewService(db *Database) *Service {
	return &Service{db: db}
}

// GetItemHandler handles GET /api/v1/inventory/:id
func (s *Service) GetItemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["id"]

	item, err := s.db.GetItem(itemID)
	if err != nil {
		if err == models.ErrItemNotFound {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	respondWithJSON(w, item)
}

// GetAllItemsHandler handles GET /api/v1/inventory/all
func (s *Service) GetAllItemsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.GetAllItems()
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	snapshot := models.InventorySnapshot{Items: items}
	respondWithJSON(w, snapshot)
}

// CheckoutHandler handles POST /api/v1/checkout
func (s *Service) CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	var req models.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !validateCheckoutRequest(req, w) {
		return
	}

	success, current, err := s.db.CheckoutWithCAS(req.ItemID, req.Quantity, req.ExpectedVersion)
	if err != nil {
		http.Error(w, fmt.Sprintf("Checkout error: %v", err), http.StatusInternalServerError)
		return
	}

	resp := buildCheckoutResponse(success, current, req.Quantity)
	respondWithJSON(w, resp)
}

// CreateOrUpdateItemHandler handles POST /api/v1/admin/inventory (for testing/demo)
func (s *Service) CreateOrUpdateItemHandler(w http.ResponseWriter, r *http.Request) {
	var item models.InventoryItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		slog.Warn("invalid JSON in admin create/update request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !validateAndNormalizeInventoryItem(&item, w) {
		return
	}

	if err := s.db.SetItem(item); err != nil {
		slog.Error("failed to set item in database", "item_id", item.ItemID, "error", err)
		http.Error(w, "Failed to set item", http.StatusInternalServerError)
		return
	}

	slog.Info("item created/updated via admin API",
		"item_id", item.ItemID,
		"quantity", item.Quantity,
		"version", item.Version)

	respondWithJSONStatus(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Item created/updated successfully",
		"item":    item,
	})
}

// HealthHandler handles GET /health
func (s *Service) HealthHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, map[string]string{
		"status":  "healthy",
		"service": "inventory-service-a",
	})
}

// Helper functions

func respondWithJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(w, `{"error":"encoding failed"}`)
	}
}

func respondWithJSONStatus(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func validateCheckoutRequest(req models.CheckoutRequest, w http.ResponseWriter) bool {
	if req.ItemID == "" {
		http.Error(w, "item_id is required", http.StatusBadRequest)
		return false
	}
	if req.Quantity <= 0 {
		http.Error(w, "quantity must be positive", http.StatusBadRequest)
		return false
	}
	if req.ExpectedVersion < 0 {
		http.Error(w, "expected_version must be non-negative", http.StatusBadRequest)
		return false
	}
	return true
}

func validateAndNormalizeInventoryItem(item *models.InventoryItem, w http.ResponseWriter) bool {
	if item.ItemID == "" {
		http.Error(w, "item_id is required", http.StatusBadRequest)
		return false
	}
	if item.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return false
	}
	if item.Quantity < 0 {
		http.Error(w, "quantity must be non-negative", http.StatusBadRequest)
		return false
	}
	if item.Version <= 0 {
		item.Version = 1
	}
	return true
}

func buildCheckoutResponse(success bool, current models.InventoryItem, requestedQty int) models.CheckoutResponse {
	resp := models.CheckoutResponse{
		Success:         success,
		CurrentVersion:  current.Version,
		CurrentQuantity: current.Quantity,
	}

	if !success {
		if current.Quantity < requestedQty {
			resp.InsufficientStock = true
			resp.Message = "Insufficient stock available"
		} else {
			resp.VersionConflict = true
			resp.Message = "Version conflict - item was modified by another operation"
		}
	} else {
		resp.Message = "Checkout successful"
	}

	return resp
}

// SetupRoutes configures all routes for the inventory service
func (s *Service) SetupRoutes(r *mux.Router) {
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/inventory/all", s.GetAllItemsHandler).Methods("GET")
	api.HandleFunc("/inventory/{id}", s.GetItemHandler).Methods("GET")
	api.HandleFunc("/checkout", s.CheckoutHandler).Methods("POST")
	api.HandleFunc("/admin/inventory", s.CreateOrUpdateItemHandler).Methods("POST")
	r.HandleFunc("/health", s.HealthHandler).Methods("GET")
}
