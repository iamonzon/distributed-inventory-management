package store

import (
	"fmt"
	"testing"

	"distributed-inventory-management/pkg/models"
)

// BenchmarkCache_Get measures cache read latency (cache hit scenario).
// This is the most frequent operation - reading from local cache.
func BenchmarkCache_Get(b *testing.B) {
	cache := NewCache()

	item := models.InventoryItem{
		ItemID:   "SKU-CACHE",
		Name:     "Cache Item",
		Quantity: 100,
		Version:  1,
	}
	cache.Set("SKU-CACHE", item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, exists := cache.Get("SKU-CACHE")
		if !exists {
			b.Fatal("Item should exist in cache")
		}
	}
}

// BenchmarkCache_Get_Miss measures cache miss latency.
func BenchmarkCache_Get_Miss(b *testing.B) {
	cache := NewCache()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, exists := cache.Get("NONEXISTENT")
		if exists {
			b.Fatal("Item should not exist in cache")
		}
	}
}

// BenchmarkCache_Set measures cache write latency.
func BenchmarkCache_Set(b *testing.B) {
	cache := NewCache()

	item := models.InventoryItem{
		ItemID:   "SKU-SET",
		Name:     "Set Item",
		Quantity: 100,
		Version:  1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(fmt.Sprintf("SKU-%d", i), item)
	}
}

// BenchmarkCache_SetAll measures bulk cache update latency.
// This simulates the polling refresh operation.
func BenchmarkCache_SetAll(b *testing.B) {
	cache := NewCache()

	// Create 100 items (typical inventory size)
	items := make([]models.InventoryItem, 100)
	for i := 0; i < 100; i++ {
		items[i] = models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%03d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: 50,
			Version:  1,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.SetAll(items)
	}
}

// BenchmarkCache_GetAll measures reading all cached items.
func BenchmarkCache_GetAll(b *testing.B) {
	cache := NewCache()

	// Seed with 100 items
	items := make([]models.InventoryItem, 100)
	for i := 0; i < 100; i++ {
		items[i] = models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%03d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: 50,
			Version:  1,
		}
	}
	cache.SetAll(items)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := cache.GetAll()
		if len(result) != 100 {
			b.Fatalf("Expected 100 items, got %d", len(result))
		}
	}
}

// BenchmarkCache_ConcurrentReads measures cache performance under concurrent reads.
// This simulates multiple concurrent browse requests.
func BenchmarkCache_ConcurrentReads(b *testing.B) {
	cache := NewCache()

	// Seed with items
	for i := 0; i < 100; i++ {
		item := models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%03d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: 50,
			Version:  1,
		}
		cache.Set(item.ItemID, item)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Read random item (using modulo to cycle through)
			cache.Get(fmt.Sprintf("SKU-%03d", b.N%100))
		}
	})
}

// BenchmarkCache_ConcurrentReadWrite measures cache performance under mixed load.
// This simulates browsing + cache updates from polling.
func BenchmarkCache_ConcurrentReadWrite(b *testing.B) {
	cache := NewCache()

	// Seed with items
	for i := 0; i < 100; i++ {
		item := models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%03d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: 50,
			Version:  1,
		}
		cache.Set(item.ItemID, item)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				// 10% writes (simulates polling updates)
				item := models.InventoryItem{
					ItemID:   fmt.Sprintf("SKU-%03d", i%100),
					Name:     "Updated Item",
					Quantity: 50,
					Version:  2,
				}
				cache.Set(item.ItemID, item)
			} else {
				// 90% reads (simulates browsing)
				cache.Get(fmt.Sprintf("SKU-%03d", i%100))
			}
			i++
		}
	})
}

// BenchmarkCache_Size measures the cost of getting cache size.
func BenchmarkCache_Size(b *testing.B) {
	cache := NewCache()

	// Seed with 100 items
	for i := 0; i < 100; i++ {
		item := models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%03d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: 50,
			Version:  1,
		}
		cache.Set(item.ItemID, item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		size := cache.Size()
		if size != 100 {
			b.Fatalf("Expected size 100, got %d", size)
		}
	}
}
