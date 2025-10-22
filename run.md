# How to Run the Project

## Prerequisites

- **Go 1.21 or later** ([Download](https://go.dev/dl/))
- **Git**
- **Terminal/Command Line**

## Quick Start (5 minutes)

### 1. Install Dependencies

```bash
cd distributed-inventory-management
go mod tidy
```

### 2. Run the Services

Open **three terminal windows** and run these commands:

**Terminal 1: Central Inventory Service (Service A)**
```bash
go run cmd/service-a/main.go
```
Expected output: `Service A running on :8080`

**Terminal 2: Store Service (Service B) - Demo Mode**
```bash
go run cmd/service-b/main.go -interval=1s
```
Expected output: `Service B running on :8081`

> **Note:** The `-interval=1s` flag is required for the demo. Production uses 30s polling (default).

**Terminal 3: Run the Demo**
```bash
go run cmd/demo/main.go
```

### 3. Expected Demo Output

```
=== Distributed Inventory Management Demo ===

Waiting for services to be ready...
Services are ready!
Service B cache initialized with 5 items!

=== Demo 1: Normal Checkout ===
✓ PASS: Checkout completed in 982.084µs

=== Demo 2: Concurrent Last Item ===
Concurrent checkout completed in 2.593167ms
Success count: 1/10
✓ PASS: Exactly one checkout succeeded
✓ PASS: Final quantity is 0

=== Demo 3: Cache Synchronization ===
Waiting for cache refresh (1.5s)...
✓ PASS: Cache synchronized successfully
```

**Success:** All 3 tests should show ✓ PASS

---

## Running Tests

### Run All Tests
```bash
go test ./...
```

### Run with Coverage
```bash
go test ./... -cover
```

Expected: **26/26 tests passing** with **75%+ coverage**

### Run Specific Test Suites

```bash
# Core CAS operations
go test ./pkg/inventory/...

# Cache and retry logic
go test ./pkg/store/...

# Integration tests
go test ./tests/integration/...

# Concurrent scenarios (critical - proves no overselling)
go test ./tests/concurrent/...

# Chaos engineering
go test ./tests/chaos/...
```

### Run Performance Benchmarks

```bash
# All benchmarks
env TEST_LOG_SILENT=true go test ./pkg/... -bench=. -benchmem

# CAS operations only
env TEST_LOG_SILENT=true go test ./pkg/inventory -bench=BenchmarkCheckoutWithCAS

# Cache operations only
env TEST_LOG_SILENT=true go test ./pkg/store -bench=BenchmarkCache
```

---

## Production Mode (30-second polling)

To run with production-like polling interval:

**Terminal 1: Service A**
```bash
go run cmd/service-a/main.go
```

**Terminal 2: Service B (production mode)**
```bash
go run cmd/service-b/main.go
# Uses default 30-second polling interval
```

---

## Health Checks

Verify services are running:

```bash
# Check Service A
curl http://localhost:8080/health
# Expected: {"status":"healthy","timestamp":"..."}

# Check Service B
curl http://localhost:8081/health
# Expected: {"status":"healthy","cache_size":N,"timestamp":"..."}
```

---

## Manual API Testing

### Get Inventory Item
```bash
curl http://localhost:8080/inventory/item1
```

### Get All Inventory
```bash
curl http://localhost:8080/inventory/all
```

### Checkout via Service A (Direct)
```bash
curl -X POST http://localhost:8080/checkout \
  -H "Content-Type: application/json" \
  -d '{"item_id":"item1","quantity":1,"version":1}'
```

### Checkout via Service B (With Retry Logic)
```bash
curl -X POST http://localhost:8081/store/checkout \
  -H "Content-Type: application/json" \
  -d '{"item_id":"item1","quantity":1}'
```

---

## Troubleshooting

### Issue: Demo fails with "Cache not synchronized"

**Cause:** Service B is using 30-second polling but demo expects 1-second polling

**Solution:** Restart Service B with the `-interval=1s` flag:
```bash
# Kill Service B (Ctrl+C)
go run cmd/service-b/main.go -interval=1s
```

### Issue: "Address already in use"

**Cause:** Services are already running

**Solution:** Kill existing processes:
```bash
# macOS/Linux
pkill -f "cmd/service"

# Or find and kill manually
lsof -ti:8080,8081 | xargs kill
```

### Issue: Tests fail with database errors

**Cause:** Previous test run left files

**Solution:** Clean and retry:
```bash
go clean -testcache
rm -f *.db
go test ./...
```

---

## Project Structure

```
cmd/
  service-a/    - Central inventory service (port 8080)
  service-b/    - Store service (port 8081)
  demo/         - Demonstration scenarios

pkg/
  inventory/    - Core CAS operations
  store/        - Cache, polling, retry logic
  models/       - Shared data structures

tests/
  integration/  - End-to-end checkout flows
  concurrent/   - Race condition tests
  chaos/        - Failure scenario tests
```

---

## Next Steps

- **Read [README.md](README.md)** for project overview
- **Read [docs/IMPLEMENTED_SOLUTION.md](docs/IMPLEMENTED_SOLUTION.md)** for architecture details
- **Read [docs/DECISION_MATRIX.md](docs/DECISION_MATRIX.md)** for design rationale
- **Read [BENCHMARK_RESULTS.md](BENCHMARK_RESULTS.md)** for performance data

---

## Service URLs

- **Service A (Central Inventory):** http://localhost:8080
- **Service B (Store Service):** http://localhost:8081

## Stop Services

Press `Ctrl+C` in each terminal window, or:

```bash
pkill -f "cmd/service"
```
