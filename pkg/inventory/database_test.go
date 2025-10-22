package inventory

import (
	"sync"
	"testing"

	"distributed-inventory-management/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabase_CAS_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Setup: Item with quantity 10, version 1
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	err := db.SetItem(item)
	require.NoError(t, err)

	// Test: Successful checkout of 3 items
	success, current, err := db.CheckoutWithCAS("SKU-123", 3, 1)

	assert.NoError(t, err)
	assert.True(t, success)
	// On success, CheckoutWithCAS now returns the updated item
	assert.Equal(t, "SKU-123", current.ItemID)
	assert.Equal(t, 7, current.Quantity)
	assert.Equal(t, 2, current.Version)

	// Verify: Quantity reduced to 7, version incremented to 2
	updated, err := db.GetItem("SKU-123")
	require.NoError(t, err)
	assert.Equal(t, 7, updated.Quantity)
	assert.Equal(t, 2, updated.Version)
}

func TestDatabase_CAS_VersionConflict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Setup: Item with quantity 10, version 1
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	err := db.SetItem(item)
	require.NoError(t, err)

	// Simulate version conflict: try to checkout with old version
	success, current, err := db.CheckoutWithCAS("SKU-123", 3, 0) // Wrong version

	assert.NoError(t, err)
	assert.False(t, success)
	assert.Equal(t, "SKU-123", current.ItemID)
	assert.Equal(t, 10, current.Quantity)
	assert.Equal(t, 1, current.Version)
}

func TestDatabase_CAS_InsufficientStock(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Setup: Item with quantity 2, version 1
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 2,
		Version:  1,
	}
	err := db.SetItem(item)
	require.NoError(t, err)

	// Test: Try to checkout more than available
	success, current, err := db.CheckoutWithCAS("SKU-123", 5, 1)

	assert.NoError(t, err)
	assert.False(t, success)
	assert.Equal(t, "SKU-123", current.ItemID)
	assert.Equal(t, 2, current.Quantity)
	assert.Equal(t, 1, current.Version)
}

func TestDatabase_CAS_ConcurrentWrites(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Setup: Item with quantity 1 (last item scenario)
	item := models.InventoryItem{
		ItemID:   "SKU-LAST",
		Name:     "Last Item",
		Quantity: 1,
		Version:  1,
	}
	err := db.SetItem(item)
	require.NoError(t, err)

	// 10 goroutines try to buy the last item
	numGoroutines := 10
	var wg sync.WaitGroup
	results := make([]bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			success, _, _ := db.CheckoutWithCAS("SKU-LAST", 1, 1)
			results[idx] = success
		}(i)
	}

	wg.Wait()

	// Verify: Exactly 1 success
	successCount := 0
	for _, success := range results {
		if success {
			successCount++
		}
	}

	assert.Equal(t, 1, successCount, "Exactly one checkout must succeed")

	// Verify: Final quantity is 0
	finalItem, err := db.GetItem("SKU-LAST")
	require.NoError(t, err)
	assert.Equal(t, 0, finalItem.Quantity)
}

func TestDatabase_GetItem_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.GetItem("NONEXISTENT")
	assert.Equal(t, models.ErrItemNotFound, err)
}

func TestDatabase_GetAllItems(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Setup: Multiple items
	items := []models.InventoryItem{
		{ItemID: "SKU-1", Name: "Item 1", Quantity: 10, Version: 1},
		{ItemID: "SKU-2", Name: "Item 2", Quantity: 20, Version: 1},
		{ItemID: "SKU-3", Name: "Item 3", Quantity: 30, Version: 1},
	}

	for _, item := range items {
		err := db.SetItem(item)
		require.NoError(t, err)
	}

	// Test: Get all items
	allItems, err := db.GetAllItems()
	require.NoError(t, err)
	assert.Len(t, allItems, 3)

	// Verify items are sorted by item_id
	assert.Equal(t, "SKU-1", allItems[0].ItemID)
	assert.Equal(t, "SKU-2", allItems[1].ItemID)
	assert.Equal(t, "SKU-3", allItems[2].ItemID)
}

func setupTestDB(t *testing.T) *Database {
	db, err := NewDatabase(":memory:")
	require.NoError(t, err)
	return db
}
