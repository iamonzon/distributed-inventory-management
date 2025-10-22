package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"distributed-inventory-management/pkg/models"
)

// Poller handles periodic synchronization with Service A
type Poller struct {
	cache    *Cache
	serviceA string
	client   *http.Client
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
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
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
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
		slog.Error("failed to fetch inventory",
			"error", err,
			"service_a", p.serviceA)
		return
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
