# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Personal Preferences
<config>
<critical_thinking_protocol>

1. ASSUMPTION CHALLENGE
- Question unstated assumptions
- Identify historical precedents being ignored
- Challenge if we're solving symptoms vs root causes
- Examine implicit contextual factors

2. SCALE IMPACT
- 10x scale behavior
- 1/10th scale behavior 
- Breaking points under pressure
- Emergent dependencies

3. CONTRARIAN VIEWS
- Cross-domain expert perspective
- Radically different context approach
- Alternative paradigms
- Strongest counter-arguments

4. TEMPORAL ANALYSIS
- Short vs long-term tradeoffs
- Historical pattern recognition
- Future evolution concerns
- Accumulated debt (technical/social/cultural)

5. COST-BENEFIT DEPTH
- Non-obvious costs
- Hidden burden bearers
- Second-order effects
- Opportunity costs

6. BIAS DETECTION
- Confirmation bias check
- Survivorship bias analysis
- Recency bias evaluation
- Correlation vs causation

7. STAKEHOLDER MATRIX
- Benefit vs burden distribution
- Missing perspectives
- Externalities
- Persona impact analysis

</critical_thinking_protocol>

<interaction_rules>
1. Never praise without specific analysis
2. Always consider multiple dimensions
3. Challenge core assumptions first
4. Propose alternative paradigms
5. Identify potential failure modes
6. Question if complexity is warranted
7. Examine hidden implications
</interaction_rules>

<response_framework>
1. Identify core assumptions
2. Apply relevant protocols
3. Present counter-perspectives
4. Analyze failure modes
5. Suggest alternative approaches
6. Question complexity/simplicity balance
7. Examine long-term implications
</response_framework>

</config>

## Project Overview

Distributed inventory management system solving 15-minute sync delays through **optimistic concurrency control (CAS) with optimized polling**. The system uses a client-server architecture with two services:

- **Service A (Central Inventory)**: Source of truth with SQLite database, handles atomic CAS operations
- **Service B (Store Service)**: Caches inventory locally, polls every 30s, handles checkout with retry logic

**Key Design**: 30-second polling + atomic CAS operations achieve 93% latency reduction with zero overselling for 10-100 stores.

## Common Commands

### Running Services

Start services in separate terminals:

```bash
# Terminal 1: Central Inventory Service
go run cmd/service-a/main.go

# Terminal 2: Store Service
go run cmd/service-b/main.go

# Terminal 3: Run demonstration
go run cmd/demo/main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run specific test suites
go test ./pkg/inventory/...     # CAS operations unit tests
go test ./pkg/store/...         # Cache and retry logic unit tests
go test ./tests/integration/... # End-to-end integration tests
go test ./tests/concurrent/...  # Concurrent scenario tests
go test ./tests/chaos/...       # Chaos engineering tests

# Run with coverage
go test -cover ./...
```

### Service URLs

- Service A: http://localhost:8080
- Service B: http://localhost:8081

Health checks:
```bash
curl http://localhost:8080/health
curl http://localhost:8081/health
```

## Architecture

### Core Components

**pkg/inventory/** - Central inventory service logic
- `database.go`: SQLite operations with WAL mode, mutex-protected reads, CAS implementation
  - CheckoutWithCAS (lines 154-203): Atomic compare-and-swap for inventory updates
- `handlers.go`: HTTP handlers for GET /inventory/:id, GET /inventory/all, POST /checkout

**pkg/store/** - Store service logic
- `cache.go`: Thread-safe in-memory cache with RWMutex
- `poller.go`: 30-second ticker-based polling from Service A
- `checkout.go`: Retry logic with exponential backoff (max 5 attempts)
- `handlers.go`: HTTP handlers for GET /store/inventory/:id, POST /store/checkout

**pkg/models/** - Shared data structures
- `inventory.go`: InventoryItem, CheckoutRequest, CheckoutResponse
- `errors.go`: ErrItemNotFound, ErrOutOfStock, ErrMaxRetriesExceeded, etc.

**cmd/** - Service entry points
- `service-a/main.go`: Starts central inventory on :8080
- `service-b/main.go`: Starts store service on :8081
- `demo/main.go`: Demonstrates concurrent scenarios

**tests/** - Test suites
- `integration/`: End-to-end checkout flows
- `concurrent/`: Race condition tests (e.g., last item scenarios)
- `chaos/`: Service failure simulations

### Key Patterns

**Compare-And-Swap (CAS)**: Atomic version-based updates prevent overselling
```sql
UPDATE inventory
SET quantity = quantity - ?, version = version + 1
WHERE item_id = ? AND version = ? AND quantity >= ?
```
Location: pkg/inventory/database.go:154-203

**Exponential Backoff with Jitter**: Prevents thundering herd during version conflicts
```go
maxBackoff := 50 * time.Millisecond * (1 << attempt)
actualBackoff := time.Duration(rand.Int63n(int64(maxBackoff)))
```
Location: pkg/store/checkout.go:84-85

**Polling**: Service B polls Service A every 30 seconds for inventory snapshot
Location: pkg/store/poller.go:41-61

### Consistency Guarantees

- **Checkout**: Strong consistency via CAS - exactly one succeeds in concurrent scenarios
- **Browsing**: Eventual consistency - cache may be up to 30s stale
- **No overselling**: Enforced by atomic `quantity >= ?` check in CAS operation

### Critical Implementation Details

1. **SQLite Configuration** (pkg/inventory/database.go:40-47):
   - WAL mode for better concurrency
   - 30-second busy timeout for lock contention
   - RWMutex at application level for additional safety

2. **Retry Strategy** (pkg/store/checkout.go:34-113):
   - Max 5 attempts per checkout
   - Version conflicts trigger cache update + retry
   - Insufficient stock returns immediately (no retry)
   - HTTP errors retry up to max attempts

3. **Cache Management** (pkg/store/poller.go):
   - Initial fetch on startup
   - Ticker-based refresh every 30s
   - Context-based graceful shutdown
   - Errors logged but don't stop polling

## Testing Approach

**Unit Tests**: CAS logic, retry mechanisms isolated
**Integration Tests**: Full checkout flows with both services running
**Concurrent Tests**: Race conditions (e.g., 10 stores competing for last item)
**Chaos Tests**: Service failures, network delays

Target coverage: 87% (excluding demo code)

## Scale Limits & Migration

**Current Design**: Appropriate for 10-100 stores, <100 concurrent checkouts/sec

**Migration Trigger**: Switch to event-driven architecture when:
- Store count exceeds 100
- Checkout rate exceeds 50/sec sustained
- Sub-second latency becomes SLA requirement

See docs/EVENT_DRIVEN_ALTERNATIVE.md for migration path.

## Key Documentation

- **docs/IMPLEMENTED_SOLUTION.md**: Architecture details and implementation walkthrough
- **docs/API.md**: REST API specification
- **docs/DECISION_MATRIX.md**: Why polling over events for this scale
- **docs/EVENT_DRIVEN_ALTERNATIVE.md**: Event-sourcing architecture for 500+ stores
