// Package store implements the store service (Service B) with caching and retry logic.
//
// This package provides:
//   - Thread-safe in-memory cache for inventory items
//   - Periodic polling from Service A for cache synchronization
//   - Checkout operations with exponential backoff retry on version conflicts
//   - Full jitter to prevent thundering herd during high contention
//
// The cache is eventually consistent (up to 30 seconds stale), but checkout
// operations use CAS against Service A for strong consistency guarantees.
package store

import (
	"distributed-inventory-management/pkg/models"
	"sync"
)

// Cache represents a thread-safe in-memory cache for inventory items
type Cache struct {
	items map[string]models.InventoryItem
	mutex sync.RWMutex
}

// NewCache creates a new inventory cache
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]models.InventoryItem),
	}
}

// Get retrieves an inventory item by ID
func (c *Cache) Get(itemID string) (models.InventoryItem, bool) {
	// METRIC: cache_lookups_total (counter, labels: result=[hit|miss])
	// Production: Track cache hit rate (should be >95% for browsing traffic)

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[itemID]
	return item, exists
}

// Set stores an inventory item
func (c *Cache) Set(itemID string, item models.InventoryItem) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[itemID] = item
}

// SetAll performs a bulk update of all cached items
func (c *Cache) SetAll(items []models.InventoryItem) {
	// METRIC: cache_refresh_items_count (gauge)
	// Production: Track catalog size growth over time

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Clear existing items
	c.items = make(map[string]models.InventoryItem)

	// Add new items
	for _, item := range items {
		c.items[item.ItemID] = item
	}
}

// GetAll returns all cached inventory items
func (c *Cache) GetAll() []models.InventoryItem {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	items := make([]models.InventoryItem, 0, len(c.items))
	for _, item := range c.items {
		items = append(items, item)
	}

	return items
}

// Size returns the number of cached items
func (c *Cache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.items)
}

// Clear removes all cached items
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]models.InventoryItem)
}
