// Package models defines shared data structures for the distributed inventory system.
//
// This package contains:
//   - InventoryItem: Core inventory data with version field for optimistic concurrency
//   - CheckoutRequest/Response: API contracts for CAS operations
//   - Custom errors for business logic failures
//
// The version field in InventoryItem is critical for the Compare-And-Swap implementation.
package models

// InventoryItem represents an item in the inventory system
type InventoryItem struct {
	ItemID   string `json:"item_id" db:"item_id"`
	Name     string `json:"name" db:"name"`
	Quantity int    `json:"quantity" db:"quantity"`
	Version  int    `json:"version" db:"version"`
}

// CheckoutRequest represents a checkout request with optimistic concurrency control
type CheckoutRequest struct {
	ItemID          string `json:"item_id"`
	Quantity        int    `json:"quantity"`
	ExpectedVersion int    `json:"expected_version"`
}

// CheckoutResponse represents the result of a checkout operation
type CheckoutResponse struct {
	Success           bool   `json:"success"`
	VersionConflict   bool   `json:"version_conflict,omitempty"`
	InsufficientStock bool   `json:"insufficient_stock,omitempty"`
	CurrentVersion    int    `json:"current_version,omitempty"`
	CurrentQuantity   int    `json:"current_quantity,omitempty"`
	Message           string `json:"message,omitempty"`
}

// InventorySnapshot represents a bulk inventory response for polling
type InventorySnapshot struct {
	Items []InventoryItem `json:"items"`
}
