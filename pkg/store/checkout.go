package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"distributed-inventory-management/pkg/models"
)

// CheckoutService handles checkout operations with retry logic
type CheckoutService struct {
	cache    *Cache
	serviceA string
	client   *http.Client
}

// NewCheckoutService creates a new checkout service
func NewCheckoutService(cache *Cache, serviceAURL string) *CheckoutService {
	return &CheckoutService{
		cache:    cache,
		serviceA: serviceAURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CheckoutWithRetry performs a checkout with exponential backoff retry logic
func (cs *CheckoutService) CheckoutWithRetry(itemID string, qty int) error {
	// Get cached item or fail fast
	cached, exists := cs.cache.Get(itemID)
	if !exists {
		return models.ErrItemNotFound
	}

	slog.Info("checkout initiated", "item_id", itemID, "quantity", qty, "version", cached.Version)

	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		start := time.Now()

		resp, err := cs.attemptCheckout(itemID, qty, cached.Version)
		if err != nil {
			// Log and fail on last attempt, otherwise continue retrying
			slog.Warn("checkout attempt failed", "item_id", itemID, "attempt", attempt+1, "error", err)
			if attempt == maxRetries-1 {
				return err
			}
			continue
		}

		if resp.Success {
			duration := time.Since(start)
			slog.Info("checkout complete", "item_id", itemID, "duration_ms", duration.Milliseconds())
			cs.updateCacheAfterSuccess(itemID, &cached, resp)
			return nil
		}

		if resp.VersionConflict {
			cs.handleVersionConflict(itemID, attempt, &cached, resp)
			continue
		}

		if resp.InsufficientStock {
			slog.Info("insufficient stock", "item_id", itemID, "requested", qty, "available", resp.CurrentQuantity)
			return models.ErrOutOfStock
		}

		if resp.Message != "" {
			slog.Error("checkout failed", "item_id", itemID, "message", resp.Message)
		}
	}

	return models.ErrMaxRetriesExceeded
}

func (cs *CheckoutService) updateCacheAfterSuccess(itemID string, cached *models.InventoryItem, resp models.CheckoutResponse) {
	if resp.CurrentVersion > 0 && resp.CurrentQuantity >= 0 {
		cached.Version = resp.CurrentVersion
		cached.Quantity = resp.CurrentQuantity
		cs.cache.Set(itemID, *cached)
	}
}

func (cs *CheckoutService) handleVersionConflict(itemID string, attempt int, cached *models.InventoryItem, resp models.CheckoutResponse) {
	slog.Debug("version conflict detected",
		"item_id", itemID,
		"attempt", attempt+1,
		"expected_version", cached.Version,
		"current_version", resp.CurrentVersion)

	// Update cache with server's current state
	cached.Version = resp.CurrentVersion
	cached.Quantity = resp.CurrentQuantity
	cs.cache.Set(itemID, *cached)

	// Exponential backoff with jitter before retry
	backoff := cs.calculateExponentialBackoffWithJitter(attempt)
	slog.Debug("retrying after backoff",
		"item_id", itemID,
		"next_attempt", attempt+2,
		"backoff_ms", backoff.Milliseconds())
	time.Sleep(backoff)
}

func (cs *CheckoutService) calculateExponentialBackoffWithJitter(attempt int) time.Duration {
	maxBackoff := 50 * time.Millisecond * (1 << attempt)
	return time.Duration(rand.Int64N(int64(maxBackoff)))
}

// attemptCheckout makes a single checkout attempt to Service A
func (cs *CheckoutService) attemptCheckout(itemID string, qty int, expectedVersion int) (models.CheckoutResponse, error) {
	req := models.CheckoutRequest{
		ItemID:          itemID,
		Quantity:        qty,
		ExpectedVersion: expectedVersion,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return models.CheckoutResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/checkout", cs.serviceA)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return models.CheckoutResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := cs.client.Do(httpReq)
	if err != nil {
		return models.CheckoutResponse{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.CheckoutResponse{}, fmt.Errorf("service A returned status %d", resp.StatusCode)
	}

	var checkoutResp models.CheckoutResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkoutResp); err != nil {
		return models.CheckoutResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return checkoutResp, nil
}
