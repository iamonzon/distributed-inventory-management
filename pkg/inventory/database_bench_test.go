package inventory

import (
	"fmt"
	"testing"

	"distributed-inventory-management/pkg/models"
)

// BenchmarkCheckoutWithCAS_Success measures the latency of successful CAS operations.
// This is the critical path for all checkout operations.
func BenchmarkCheckoutWithCAS_Success(b *testing.B) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Seed item with high quantity to avoid stock issues
	item := models.InventoryItem{
		ItemID:   "SKU-BENCH",
		Name:     "Benchmark Item",
		Quantity: 1000000, // 1M items
		Version:  1,
	}
	if err := db.SetItem(item); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Each iteration checkouts 1 item
		// Version increments each time, so we need to track it
		version := i + 1
		_, _, err := db.CheckoutWithCAS("SKU-BENCH", 1, version)
		if err != nil {
			b.Fatalf("Checkout failed: %v", err)
		}
	}
}

// BenchmarkCheckoutWithCAS_VersionConflict measures CAS performance under version conflicts.
// This simulates the retry scenario.
func BenchmarkCheckoutWithCAS_VersionConflict(b *testing.B) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	item := models.InventoryItem{
		ItemID:   "SKU-CONFLICT",
		Name:     "Conflict Item",
		Quantity: 1000000,
		Version:  1,
	}
	if err := db.SetItem(item); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Always use wrong version to trigger conflict
		success, _, err := db.CheckoutWithCAS("SKU-CONFLICT", 1, 999)
		if err != nil {
			b.Fatalf("Checkout failed: %v", err)
		}
		if success {
			b.Fatal("Expected version conflict but got success")
		}
	}
}

// BenchmarkGetItem measures single item read latency.
// This is the cache miss scenario where we read from the database.
func BenchmarkGetItem(b *testing.B) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	item := models.InventoryItem{
		ItemID:   "SKU-READ",
		Name:     "Read Item",
		Quantity: 100,
		Version:  1,
	}
	if err := db.SetItem(item); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.GetItem("SKU-READ")
		if err != nil {
			b.Fatalf("GetItem failed: %v", err)
		}
	}
}

// BenchmarkGetAllItems measures bulk read performance.
// This simulates the polling operation from Service B.
func BenchmarkGetAllItems(b *testing.B) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Seed with 100 items (typical inventory size)
	for i := 1; i <= 100; i++ {
		item := models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%03d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: 50,
			Version:  1,
		}
		if err := db.SetItem(item); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := db.GetAllItems()
		if err != nil {
			b.Fatalf("GetAllItems failed: %v", err)
		}
		if len(items) != 100 {
			b.Fatalf("Expected 100 items, got %d", len(items))
		}
	}
}

// BenchmarkConcurrentCheckouts measures CAS performance under concurrent load.
// This simulates multiple stores checking out simultaneously.
func BenchmarkConcurrentCheckouts(b *testing.B) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	item := models.InventoryItem{
		ItemID:   "SKU-CONCURRENT",
		Name:     "Concurrent Item",
		Quantity: 1000000,
		Version:  1,
	}
	if err := db.SetItem(item); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// This will cause version conflicts, but that's realistic
			db.CheckoutWithCAS("SKU-CONCURRENT", 1, 1)
		}
	})
}

// BenchmarkSetItem measures write performance for admin operations.
func BenchmarkSetItem(b *testing.B) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := models.InventoryItem{
			ItemID:   fmt.Sprintf("SKU-%d", i),
			Name:     "Benchmark Item",
			Quantity: 100,
			Version:  1,
		}
		if err := db.SetItem(item); err != nil {
			b.Fatalf("SetItem failed: %v", err)
		}
	}
}
