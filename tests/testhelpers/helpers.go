package testhelpers

import (
	"fmt"
	"net"
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

// StartTestServer starts an HTTP server for testing on a dynamic port.
// Returns the server and the actual address it's listening on (e.g., "localhost:54321").
func StartTestServer(handler http.Handler) (*http.Server, string) {
	// Listen on a random available port (port 0)
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(fmt.Sprintf("Failed to create listener: %v", err))
	}

	server := &http.Server{
		Handler: handler,
	}

	go func() {
		server.Serve(listener)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	return server, listener.Addr().String()
}

// StartTestServerWithDelay starts an HTTP server with artificial delay middleware on a dynamic port.
// Returns the server and the actual address it's listening on.
func StartTestServerWithDelay(handler http.Handler, delay time.Duration) (*http.Server, string) {
	// Wrap handler with delay middleware
	delayedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		handler.ServeHTTP(w, r)
	})

	// Listen on a random available port (port 0)
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(fmt.Sprintf("Failed to create listener: %v", err))
	}

	server := &http.Server{
		Handler: delayedHandler,
	}

	go func() {
		server.Serve(listener)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	return server, listener.Addr().String()
}

// SetupTestServices creates and starts both Service A and Service B for integration tests.
// Returns the database, cache, servers, service addresses, and cleanup function.
func SetupTestServices(t *testing.T) (
	db *inventory.Database,
	cache *store.Cache,
	serverA *http.Server,
	serverB *http.Server,
	serviceAAddr string,
	serviceBAddr string,
	cleanup func(),
) {
	t.Helper()

	// Setup Service A
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)

	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA, serviceAAddr = StartTestServer(routerA)

	// Setup Service B
	cache = store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB, serviceBAddr = StartTestServer(routerB)

	cleanup = func() {
		serverA.Close()
		serverB.Close()
		db.Close()
	}

	return db, cache, serverA, serverB, serviceAAddr, serviceBAddr, cleanup
}

// SetupTestServicesWithDelay creates test services with artificial delay on Service A.
// This is useful for testing timeout and retry behavior.
// Returns the database, cache, servers, service addresses, and cleanup function.
func SetupTestServicesWithDelay(t *testing.T, delay time.Duration) (
	db *inventory.Database,
	cache *store.Cache,
	serverA *http.Server,
	serverB *http.Server,
	serviceAAddr string,
	serviceBAddr string,
	cleanup func(),
) {
	t.Helper()

	// Setup Service A with delay
	db, err := inventory.NewDatabase(":memory:")
	require.NoError(t, err)

	serviceA := inventory.NewService(db)
	routerA := mux.NewRouter()
	serviceA.SetupRoutes(routerA)
	serverA, serviceAAddr = StartTestServerWithDelay(routerA, delay)

	// Setup Service B
	cache = store.NewCache()
	checkoutSvc := store.NewCheckoutService(cache, "http://"+serviceAAddr)
	storeSvc := store.NewStoreService(cache, checkoutSvc)

	routerB := mux.NewRouter()
	storeSvc.SetupRoutes(routerB)
	serverB, serviceBAddr = StartTestServer(routerB)

	cleanup = func() {
		serverA.Close()
		serverB.Close()
		db.Close()
	}

	return db, cache, serverA, serverB, serviceAAddr, serviceBAddr, cleanup
}
