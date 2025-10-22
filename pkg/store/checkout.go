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
	cached, err := cs.getCachedItemOrFail(itemID)
	if err != nil {
		return err
	}
	cs.logCheckoutInitiated(itemID, qty, cached.Version)

	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		start := time.Now()

		resp, err := cs.attemptCheckout(itemID, qty, cached.Version)
		if err != nil {
			if cs.isLastAttempt(attempt, maxRetries) {
				return cs.logAndReturnError(itemID, attempt, err)
			}
			cs.logAttemptFailed(itemID, attempt, err)
			continue
		}

		if resp.Success {
			cs.logCheckoutComplete(itemID, start)
			cs.updateCacheAfterSuccess(itemID, &cached, resp)
			return nil
		}

		if resp.VersionConflict {
			cs.handleVersionConflict(itemID, attempt, &cached, resp)
			continue
		}

		if resp.InsufficientStock {
			return cs.handleInsufficientStock(itemID, qty, resp.CurrentQuantity)
		}

		if resp.Message != "" {
			slog.Error("checkout failed", "item_id", itemID, "message", resp.Message)
		}
	}

	return models.ErrMaxRetriesExceeded
}

func (cs *CheckoutService) getCachedItemOrFail(itemID string) (models.InventoryItem, error) {
	cached, exists := cs.cache.Get(itemID)
	if !exists {
		return models.InventoryItem{}, models.ErrItemNotFound
	}
	return cached, nil
}

func (cs *CheckoutService) logCheckoutInitiated(itemID string, qty, version int) {
	slog.Info("checkout initiated", "item_id", itemID, "quantity", qty, "version", version)
}

func (cs *CheckoutService) isLastAttempt(attempt, maxRetries int) bool {
	return attempt == maxRetries-1
}

func (cs *CheckoutService) logAndReturnError(itemID string, attempt int, err error) error {
	slog.Warn("checkout attempt failed", "item_id", itemID, "attempt", attempt+1, "error", err)
	return err
}

func (cs *CheckoutService) logAttemptFailed(itemID string, attempt int, err error) {
	slog.Warn("checkout attempt failed", "item_id", itemID, "attempt", attempt+1, "error", err)
}

func (cs *CheckoutService) logCheckoutComplete(itemID string, start time.Time) {
	duration := time.Since(start)
	slog.Info("checkout complete", "item_id", itemID, "duration_ms", duration.Milliseconds())
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

	cs.updateCacheWithServerState(itemID, cached, resp)
	backoff := cs.calculateExponentialBackoffWithJitter(attempt)
	cs.logAndSleepBeforeRetry(itemID, attempt, backoff)
}

func (cs *CheckoutService) updateCacheWithServerState(itemID string, cached *models.InventoryItem, resp models.CheckoutResponse) {
	cached.Version = resp.CurrentVersion
	cached.Quantity = resp.CurrentQuantity
	cs.cache.Set(itemID, *cached)
}

func (cs *CheckoutService) calculateExponentialBackoffWithJitter(attempt int) time.Duration {
	maxBackoff := 50 * time.Millisecond * (1 << attempt)
	return time.Duration(rand.Int64N(int64(maxBackoff)))
}

func (cs *CheckoutService) logAndSleepBeforeRetry(itemID string, attempt int, backoff time.Duration) {
	slog.Debug("retrying after backoff",
		"item_id", itemID,
		"next_attempt", attempt+2,
		"backoff_ms", backoff.Milliseconds())
	time.Sleep(backoff)
}

func (cs *CheckoutService) handleInsufficientStock(itemID string, requested, available int) error {
	slog.Info("insufficient stock", "item_id", itemID, "requested", requested, "available", available)
	return models.ErrOutOfStock
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
