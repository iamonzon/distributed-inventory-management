// Package inventory implements the central inventory service (Service A).
//
// This package provides Compare-And-Swap (CAS) operations for strong consistency
// in concurrent checkout scenarios. The implementation uses SQLite with WAL mode
// for better concurrency and atomic version-based updates to prevent overselling.
//
// Key guarantees:
//   - Atomic version-based updates prevent overselling
//   - Exactly one transaction succeeds when multiple compete
//   - No dirty reads (version mismatch detected immediately)
//
// The CAS implementation is in CheckoutWithCAS (database.go:142-191).
package inventory

import (
	"database/sql"
	"fmt"
	"sync"

	"distributed-inventory-management/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

// Database represents the central inventory database with CAS operations
type Database struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// NewDatabase creates a new database connection and initializes the schema
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := configureSQLitePragmas(db); err != nil {
		return nil, err
	}

	database := &Database{
		db:   db,
		path: dbPath,
	}

	if err := database.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

func configureSQLitePragmas(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout=30000"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	return nil
}

// initSchema creates the inventory table if it doesn't exist
func (d *Database) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS inventory (
		item_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		version INTEGER NOT NULL DEFAULT 1
	)`

	_, err := d.db.Exec(query)
	return err
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// SetItem creates or updates an inventory item (for testing/seeding)
func (d *Database) SetItem(item models.InventoryItem) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT OR REPLACE INTO inventory (item_id, name, quantity, version)
	VALUES (?, ?, ?, ?)`

	_, err := d.db.Exec(query, item.ItemID, item.Name, item.Quantity, item.Version)
	return err
}

// GetItem retrieves a single inventory item by ID
func (d *Database) GetItem(itemID string) (models.InventoryItem, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT item_id, name, quantity, version
	FROM inventory
	WHERE item_id = ?`

	var item models.InventoryItem
	err := d.db.QueryRow(query, itemID).Scan(
		&item.ItemID,
		&item.Name,
		&item.Quantity,
		&item.Version,
	)

	if err == sql.ErrNoRows {
		return item, models.ErrItemNotFound
	}

	return item, err
}

// GetAllItems retrieves all inventory items for bulk synchronization
func (d *Database) GetAllItems() ([]models.InventoryItem, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT item_id, name, quantity, version
	FROM inventory
	ORDER BY item_id`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.InventoryItem
	for rows.Next() {
		var item models.InventoryItem
		err := rows.Scan(
			&item.ItemID,
			&item.Name,
			&item.Quantity,
			&item.Version,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// CheckoutWithCAS performs an atomic compare-and-swap operation for inventory checkout
func (d *Database) CheckoutWithCAS(
	itemID string,
	quantity int,
	expectedVersion int,
) (success bool, current models.InventoryItem, err error) {
	if quantity <= 0 {
		return false, models.InventoryItem{}, models.ErrInvalidQuantity
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return false, models.InventoryItem{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Attempt atomic update with CAS
	result, err := tx.Exec(`
		UPDATE inventory 
		SET quantity = quantity - ?, version = version + 1
		WHERE item_id = ? AND version = ? AND quantity >= ?
	`, quantity, itemID, expectedVersion, quantity)

	if err != nil {
		return false, models.InventoryItem{}, fmt.Errorf("failed to execute CAS update: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, models.InventoryItem{}, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// CAS failed - get current state for client
		current, err := d.getItemInTx(tx, itemID)
		if err != nil {
			return false, models.InventoryItem{}, err
		}
		return false, current, nil
	}

	// CAS succeeded - get updated state before committing
	updated, err := d.getItemInTx(tx, itemID)
	if err != nil {
		return false, models.InventoryItem{}, err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return false, models.InventoryItem{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return true, updated, nil
}

// getItemInTx retrieves an item within a transaction
func (d *Database) getItemInTx(tx *sql.Tx, itemID string) (models.InventoryItem, error) {
	query := `
	SELECT item_id, name, quantity, version
	FROM inventory
	WHERE item_id = ?`

	var item models.InventoryItem
	err := tx.QueryRow(query, itemID).Scan(
		&item.ItemID,
		&item.Name,
		&item.Quantity,
		&item.Version,
	)

	if err == sql.ErrNoRows {
		return item, models.ErrItemNotFound
	}

	return item, err
}
