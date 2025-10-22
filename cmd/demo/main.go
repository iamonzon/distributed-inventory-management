package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"distributed-inventory-management/pkg/models"
)

const (
	serviceAURL = "http://localhost:8080"
	serviceBURL = "http://localhost:8081"
)

var (
	// pollInterval can be configured via POLL_INTERVAL_SECONDS env var (default 1 second for demo)
	pollInterval time.Duration
)

func init() {
	// Read polling interval from environment variable (default to 1 second for demo)
	intervalStr := os.Getenv("POLL_INTERVAL_SECONDS")
	if intervalStr == "" {
		pollInterval = 1 * time.Second
	} else {
		seconds, err := strconv.Atoi(intervalStr)
		if err != nil {
			log.Printf("Invalid POLL_INTERVAL_SECONDS value '%s', using default 1 second", intervalStr)
			pollInterval = 1 * time.Second
		} else {
			pollInterval = time.Duration(seconds) * time.Second
		}
	}
	log.Printf("Using polling interval: %v", pollInterval)
}

func main() {
	log.Println("=== Distributed Inventory Management Demo ===")
	log.Println()

	// Wait for services to be ready
	if !waitForServices() {
		log.Fatal("Services not ready after 30 seconds")
	}

	// Verify cache is populated
	if !verifyCacheInitialization() {
		log.Fatal("Service B cache not initialized after 30 seconds")
	}

	// Demo 1: Normal Checkout
	demoNormalCheckout()

	// Demo 2: Concurrent Last Item
	demoConcurrentLastItem()

	// Demo 3: Cache Synchronization
	demoCacheSynchronization()

	log.Println("\n=== Demo Complete ===")
}

func waitForServices() bool {
	log.Println("Waiting for services to be ready...")

	for i := 0; i < 30; i++ {
		if checkServiceHealth(serviceAURL) && checkServiceHealth(serviceBURL) {
			log.Println("Services are ready!")
			return true
		}
		time.Sleep(1 * time.Second)
	}

	return false
}

func checkServiceHealth(url string) bool {
	resp, err := http.Get(url + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func verifyCacheInitialization() bool {
	log.Println("Waiting for Service B cache to populate (polling from Service A)...")

	for i := 0; i < 30; i++ {
		// Check if Service B cache has any items
		resp, err := http.Get(serviceBURL + "/health")
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		defer resp.Body.Close()

		var health map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Check cache size
		if cacheSize, ok := health["cache_size"].(float64); ok && cacheSize > 0 {
			log.Printf("Service B cache initialized with %d items!", int(cacheSize))
			return true
		}

		time.Sleep(1 * time.Second)
	}

	return false
}

func demoNormalCheckout() {
	log.Println("=== Demo 1: Normal Checkout ===")

	// Checkout 1 item of SKU-123
	req := map[string]any{
		"item_id":  "SKU-123",
		"quantity": 1,
	}

	start := time.Now()
	resp, err := makeCheckoutRequest(serviceBURL, req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("✗ FAIL: Checkout failed: %v", err)
		return
	}

	if resp == nil {
		log.Printf("✗ FAIL: No response received")
		return
	}

	if success, ok := resp["success"].(bool); ok && success {
		log.Printf("✓ PASS: Checkout completed in %v", duration)
	} else {
		log.Printf("✗ FAIL: %s", resp["message"])
	}
	log.Println()
}

func demoConcurrentLastItem() {
	log.Println("=== Demo 2: Concurrent Last Item ===")

	// First, set up an item with quantity 1
	setupLastItem()

	// 10 stores try to buy the last item simultaneously
	numStores := 10
	var wg sync.WaitGroup
	results := make([]map[string]any, numStores)

	start := time.Now()

	for i := range numStores {
		wg.Add(1)
		go func(storeID int) {
			defer wg.Done()

			req := map[string]any{
				"item_id":  "SKU-LAST",
				"quantity": 1,
			}

			resp, err := makeCheckoutRequest(serviceBURL, req)
			if err != nil {
				results[storeID] = map[string]any{
					"success": false,
					"error":   err.Error(),
				}
			} else if resp != nil {
				results[storeID] = resp
			} else {
				results[storeID] = map[string]any{
					"success": false,
					"error":   "nil response received",
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Count successes
	successCount := 0
	for _, result := range results {
		// Check if result is nil or if success field exists and is true
		if result != nil {
			if success, ok := result["success"].(bool); ok && success {
				successCount++
			}
		}
	}

	log.Printf("Concurrent checkout completed in %v", duration)
	log.Printf("Success count: %d/%d", successCount, numStores)

	if successCount == 1 {
		log.Printf("✓ PASS: Exactly one checkout succeeded")
	} else {
		log.Printf("✗ FAIL: Expected 1 success, got %d", successCount)
	}

	// Verify final quantity is 0
	finalItem := getItemFromServiceA("SKU-LAST")
	if finalItem == nil {
		log.Printf("✗ FAIL: Could not retrieve final item state")
	} else if qty, ok := finalItem["quantity"].(float64); ok && qty == 0 {
		log.Printf("✓ PASS: Final quantity is 0")
	} else {
		log.Printf("✗ FAIL: Final quantity is %v, expected 0", finalItem["quantity"])
	}
	log.Println()
}

func demoCacheSynchronization() {
	log.Println("=== Demo 3: Cache Synchronization ===")

	// Update an item in Service A
	updateItemInServiceA("SKU-123", "Updated Wireless Headphones", 75, 2)

	// Wait for cache refresh (polling interval + buffer)
	waitTime := pollInterval + (500 * time.Millisecond)
	log.Printf("Waiting for cache refresh (%v)...", waitTime)
	time.Sleep(waitTime)

	// Check if Service B cache has the updated item
	cachedItem := getItemFromServiceB("SKU-123")

	if cachedItem == nil {
		log.Printf("✗ FAIL: Could not retrieve cached item")
	} else {
		name, nameOk := cachedItem["name"].(string)
		quantity, quantityOk := cachedItem["quantity"].(float64)
		version, versionOk := cachedItem["version"].(float64)

		if nameOk && quantityOk && versionOk &&
			name == "Updated Wireless Headphones" &&
			quantity == 75 &&
			version == 2 {
			log.Printf("✓ PASS: Cache synchronized successfully")
		} else {
			log.Printf("✗ FAIL: Cache not synchronized")
			log.Printf("Expected: name='Updated Wireless Headphones', quantity=75, version=2")
			log.Printf("Got: name='%v', quantity=%v, version=%v",
				cachedItem["name"], cachedItem["quantity"], cachedItem["version"])
		}
	}
	log.Println()
}

func setupLastItem() {
	// Create an item with quantity 1 for the concurrent test
	item := models.InventoryItem{
		ItemID:   "SKU-LAST",
		Name:     "Last Item",
		Quantity: 1,
		Version:  1,
	}

	// Add this item to Service A via admin API
	log.Printf("Setting up last item: %s", item.ItemID)

	jsonData, err := json.Marshal(item)
	if err != nil {
		log.Fatalf("Failed to marshal item: %v", err)
	}

	resp, err := http.Post(serviceAURL+"/api/v1/admin/inventory", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to create item: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		log.Fatalf("Failed to create item, status: %d", resp.StatusCode)
	}

	// Wait for Service B cache to pick up the new item via polling
	log.Println("Waiting for cache to sync new item...")
	maxWait := int((pollInterval + 2*time.Second) / time.Second)
	for i := 0; i < maxWait; i++ {
		cachedItem := getItemFromServiceB("SKU-LAST")
		if cachedItem != nil {
			log.Println("✓ SKU-LAST synchronized to Service B cache")
			return
		}
		time.Sleep(1 * time.Second)
	}
	log.Printf("⚠ Warning: SKU-LAST not yet in cache after %d seconds, proceeding anyway...", maxWait)
}

func updateItemInServiceA(itemID, name string, quantity, version int) {
	log.Printf("Updating item %s in Service A", itemID)

	item := models.InventoryItem{
		ItemID:   itemID,
		Name:     name,
		Quantity: quantity,
		Version:  version,
	}

	jsonData, err := json.Marshal(item)
	if err != nil {
		log.Fatalf("Failed to marshal item: %v", err)
	}

	resp, err := http.Post(serviceAURL+"/api/v1/admin/inventory", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to update item: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		log.Fatalf("Failed to update item, status: %d", resp.StatusCode)
	}
}

func makeCheckoutRequest(serviceURL string, req map[string]any) (map[string]any, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(serviceURL+"/store/checkout", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func getItemFromServiceA(itemID string) map[string]any {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/inventory/%s", serviceAURL, itemID))
	if err != nil {
		log.Printf("Failed to get item from Service A: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var item map[string]any
	json.NewDecoder(resp.Body).Decode(&item)
	return item
}

func getItemFromServiceB(itemID string) map[string]any {
	resp, err := http.Get(fmt.Sprintf("%s/store/inventory/%s", serviceBURL, itemID))
	if err != nil {
		log.Printf("Failed to get item from Service B: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var item map[string]any
	json.NewDecoder(resp.Body).Decode(&item)
	return item
}
