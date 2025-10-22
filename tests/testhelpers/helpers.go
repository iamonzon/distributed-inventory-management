package testhelpers

import (
	"net/http"
	"testing"
	"time"

	"distributed-inventory-management/pkg/inventory"
	"distributed-inventory-management/pkg/models"
	"distributed-inventory-management/pkg/store"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

// InitializeTestCache populates the cache with items from the database.
// This simulates the result of the poller fetching data from Service A.
func InitializeTestCache(t *testing.T, cache *store.Cache, db *inventory.Database) {
	t.Helper()

	items, err := db.GetAllItems()
	require.NoError(t, err, "Failed to fetch items from database")

	cache.SetAll(items)
}

// SeedTestItem creates a test item in the database and returns it.
func SeedTestItem(t *testing.T, db *inventory.Database, itemID, name string, quantity, version int) models.InventoryItem {
	t.Helper()

	item := models.InventoryItem{
		ItemID:   itemID,
		Name:     name,
		Quantity: quantity,
		Version:  version,
	}

	err := db.SetItem(item)
	require.NoError(t, err, "Failed to seed test item")

	return item
}

// StartTestServer starts an HTTP server for testing and returns it.
func StartTestServer(addr string, handler http.Handler) *http.Server {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	return server
}

// StartTestServerWithDelay starts an HTTP server with artificial delay middleware.
func StartTestServerWithDelay(addr string, handler http.Handler, delay time.Duration) *http.Server {
	// Wrap handler with delay middleware
	delayedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		handler.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: delayedHandler,
	}

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	return server
}

// SetupTestServices creates and starts both Service A and Service B for integration tests.
// Returns the database, cache, servers, and cleanup function.
func SetupTestServices(t *testing.T, serviceAAddr, serviceBAddr string) (
	db *inventory.Database,
	cache *store.Cache,
	serverA *http.Server,
	serverB *http.Server,
	cleanup func(),
) {
	t.Helper()

	// Setup Service A
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)

	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA = StartTestServer(serviceAAddr, routerA)

	// Setup Service B
	cache = store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB = StartTestServer(serviceBAddr, routerB)

	cleanup = func() {
		serverA.Close()
		serverB.Close()
		db.Close()
	}

	return db, cache, serverA, serverB, cleanup
}

// SetupTestServicesWithDelay creates test services with artificial delay on Service A.
// This is useful for testing timeout and retry behavior.
func SetupTestServicesWithDelay(t *testing.T, serviceAAddr, serviceBAddr string, delay time.Duration) (
	db *inventory.Database,
	cache *store.Cache,
	serverA *http.Server,
	serverB *http.Server,
	cleanup func(),
) {
	t.Helper()

	// Setup Service A with delay
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)

	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA = StartTestServerWithDelay(serviceAAddr, routerA, delay)

	// Setup Service B
	cache = store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB = StartTestServer(serviceBAddr, routerB)

	cleanup = func() {
		serverA.Close()
		serverB.Close()
		db.Close()
	}

	return db, cache, serverA, serverB, cleanup
}
