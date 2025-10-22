package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"distributed-inventory-management/pkg/models"
)

// Poller handles periodic synchronization with Service A
type Poller struct {
	cache              *Cache
	serviceA           string
	client             *http.Client
	interval           time.Duration
	ctx                context.Context
	cancel             context.CancelFunc
	consecutiveFailures int
	maxBackoffInterval time.Duration
}

// NewPoller creates a new inventory poller
func NewPoller(cache *Cache, serviceAURL string, interval time.Duration) *Poller {
	ctx, cancel := context.WithCancel(context.Background())

	return &Poller{
		cache:    cache,
		serviceA: serviceAURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		interval:           interval,
		ctx:                ctx,
		cancel:             cancel,
		consecutiveFailures: 0,
		maxBackoffInterval: 5 * time.Minute, // Cap backoff at 5 minutes
	}
}

// StartPolling begins the periodic polling process
func (p *Poller) StartPolling() {
	slog.Info("starting inventory polling",
		"interval", p.interval,
		"service_a", p.serviceA)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.performInitialSync()

	for {
		select {
		case <-ticker.C:
			p.fetchAndUpdate()
		case <-p.ctx.Done():
			slog.Info("poller stopped")
			return
		}
	}
}

func (p *Poller) performInitialSync() {
	p.fetchAndUpdate()
}

// Stop stops the polling process
func (p *Poller) Stop() {
	p.cancel()
}

// fetchAndUpdate retrieves inventory from Service A and updates the cache
func (p *Poller) fetchAndUpdate() {
	start := time.Now()

	items, err := p.fetchAllInventory()
	if err != nil {
		p.consecutiveFailures++
		backoff := p.calculateBackoffWithJitter()

		slog.Error("failed to fetch inventory",
			"error", err,
			"service_a", p.serviceA,
			"consecutive_failures", p.consecutiveFailures,
			"next_retry_delay_ms", backoff.Milliseconds())

		// Sleep with backoff before next poll attempt
		// This reduces load on Service A during outages
		select {
		case <-time.After(backoff):
			return
		case <-p.ctx.Done():
			return
		}
	}

	// Success - reset failure counter
	if p.consecutiveFailures > 0 {
		slog.Info("polling recovered after failures",
			"consecutive_failures", p.consecutiveFailures)
		p.consecutiveFailures = 0
	}

	p.cache.SetAll(items)

	duration := time.Since(start)
	slog.Debug("cache refreshed",
		"duration_ms", duration.Milliseconds(),
		"item_count", len(items))
}

// fetchAllInventory retrieves all inventory items from Service A
func (p *Poller) fetchAllInventory() ([]models.InventoryItem, error) {
	url := fmt.Sprintf("%s/api/v1/inventory/all", p.serviceA)

	req, err := http.NewRequestWithContext(p.ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch inventory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service A returned status %d", resp.StatusCode)
	}

	var snapshot models.InventorySnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return snapshot.Items, nil
}

// calculateBackoffWithJitter calculates exponential backoff delay with jitter
// to prevent thundering herd when Service A is unavailable
func (p *Poller) calculateBackoffWithJitter() time.Duration {
	// Exponential backoff: base_interval * 2^failures
	// Start with the polling interval as base
	backoff := p.interval * (1 << min(p.consecutiveFailures-1, 6)) // Cap at 2^6 = 64x multiplier

	// Cap at max backoff interval (5 minutes by default)
	if backoff > p.maxBackoffInterval {
		backoff = p.maxBackoffInterval
	}

	// Add jitter: random value between 0 and calculated backoff
	// This prevents synchronized retries across multiple Service B instances
	jitteredBackoff := time.Duration(rand.Int64N(int64(backoff)))

	return jitteredBackoff
}
