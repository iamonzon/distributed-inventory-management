package store

import (
	"sync"
	"testing"

	"distributed-inventory-management/pkg/models"

	"github.com/stretchr/testify/assert"
)

func TestCache_ConcurrentReadWrite(t *testing.T) {
	cache := NewCache()

	// Setup: Add initial item
	item := models.InventoryItem{
		ItemID:   "SKU-123",
		Name:     "Test Item",
		Quantity: 10,
		Version:  1,
	}
	cache.Set("SKU-123", item)

	// Test: Concurrent reads and writes
	numGoroutines := 100
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			updatedItem := item
			updatedItem.Quantity = 10 + idx
			updatedItem.Version = 1 + idx
			cache.Set("SKU-123", updatedItem)
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, exists := cache.Get("SKU-123")
			assert.True(t, exists)
		}()
	}

	wg.Wait()

	// Verify: Item still exists and is consistent
	retrieved, exists := cache.Get("SKU-123")
	assert.True(t, exists)
	assert.Equal(t, "SKU-123", retrieved.ItemID)
}

func TestCache_SetAll(t *testing.T) {
	cache := NewCache()

	// Setup: Add some initial items
	cache.Set("SKU-1", models.InventoryItem{ItemID: "SKU-1", Name: "Item 1"})
	cache.Set("SKU-2", models.InventoryItem{ItemID: "SKU-2", Name: "Item 2"})

	// Test: Replace all items
	newItems := []models.InventoryItem{
		{ItemID: "SKU-3", Name: "Item 3", Quantity: 30, Version: 1},
		{ItemID: "SKU-4", Name: "Item 4", Quantity: 40, Version: 1},
		{ItemID: "SKU-5", Name: "Item 5", Quantity: 50, Version: 1},
	}

	cache.SetAll(newItems)

	// Verify: Only new items exist
	assert.Equal(t, 3, cache.Size())

	allItems := cache.GetAll()
	assert.Len(t, allItems, 3)

	// Verify: Old items are gone
	_, exists := cache.Get("SKU-1")
	assert.False(t, exists)

	_, exists = cache.Get("SKU-2")
	assert.False(t, exists)

	// Verify: New items exist
	item3, exists := cache.Get("SKU-3")
	assert.True(t, exists)
	assert.Equal(t, "Item 3", item3.Name)
}

func TestCache_GetAll(t *testing.T) {
	cache := NewCache()

	// Setup: Add multiple items
	items := []models.InventoryItem{
		{ItemID: "SKU-1", Name: "Item 1", Quantity: 10, Version: 1},
		{ItemID: "SKU-2", Name: "Item 2", Quantity: 20, Version: 1},
		{ItemID: "SKU-3", Name: "Item 3", Quantity: 30, Version: 1},
	}

	for _, item := range items {
		cache.Set(item.ItemID, item)
	}

	// Test: Get all items
	allItems := cache.GetAll()
	assert.Len(t, allItems, 3)

	// Verify: All items are present
	itemMap := make(map[string]models.InventoryItem)
	for _, item := range allItems {
		itemMap[item.ItemID] = item
	}

	assert.Contains(t, itemMap, "SKU-1")
	assert.Contains(t, itemMap, "SKU-2")
	assert.Contains(t, itemMap, "SKU-3")
}

func TestCache_Clear(t *testing.T) {
	cache := NewCache()

	// Setup: Add some items
	cache.Set("SKU-1", models.InventoryItem{ItemID: "SKU-1", Name: "Item 1"})
	cache.Set("SKU-2", models.InventoryItem{ItemID: "SKU-2", Name: "Item 2"})

	assert.Equal(t, 2, cache.Size())

	// Test: Clear cache
	cache.Clear()

	// Verify: Cache is empty
	assert.Equal(t, 0, cache.Size())

	_, exists := cache.Get("SKU-1")
	assert.False(t, exists)

	_, exists = cache.Get("SKU-2")
	assert.False(t, exists)
}

func TestCache_Size(t *testing.T) {
	cache := NewCache()

	// Test: Empty cache
	assert.Equal(t, 0, cache.Size())

	// Test: Add items
	cache.Set("SKU-1", models.InventoryItem{ItemID: "SKU-1"})
	assert.Equal(t, 1, cache.Size())

	cache.Set("SKU-2", models.InventoryItem{ItemID: "SKU-2"})
	assert.Equal(t, 2, cache.Size())

	// Test: Update existing item (size should remain same)
	cache.Set("SKU-1", models.InventoryItem{ItemID: "SKU-1", Name: "Updated"})
	assert.Equal(t, 2, cache.Size())
}
