# REST API Specification

## Service A (Central Inventory Service)

**Base URL:** `http://localhost:8080`

### Endpoints

#### Get Single Item
```http
GET /api/v1/inventory/{id}
```

**Description:** Retrieve a single inventory item by ID.

**Parameters:**
- `id` (path): Item ID (e.g., "SKU-123")

**Response:**
```json
{
  "item_id": "SKU-123",
  "name": "Wireless Headphones",
  "quantity": 50,
  "version": 1
}
```

**Status Codes:**
- `200 OK`: Item found
- `404 Not Found`: Item not found
- `500 Internal Server Error`: Database error

#### Get All Items
```http
GET /api/v1/inventory/all
```

**Description:** Retrieve all inventory items for bulk synchronization.

**Response:**
```json
{
  "items": [
    {
      "item_id": "SKU-123",
      "name": "Wireless Headphones",
      "quantity": 50,
      "version": 1
    },
    {
      "item_id": "SKU-456",
      "name": "Smartphone Case",
      "quantity": 100,
      "version": 1
    }
  ]
}
```

**Status Codes:**
- `200 OK`: Success
- `500 Internal Server Error`: Database error

#### Checkout (CAS Operation)
```http
POST /api/v1/checkout
```

**Description:** Perform atomic checkout with Compare-And-Swap operation.

**Request Body:**
```json
{
  "item_id": "SKU-123",
  "quantity": 2,
  "expected_version": 1
}
```

**Response (Success):**
```json
{
  "success": true,
  "message": "Checkout successful"
}
```

**Response (Version Conflict):**
```json
{
  "success": false,
  "version_conflict": true,
  "current_version": 2,
  "current_quantity": 48,
  "message": "Version conflict - item was modified by another operation"
}
```

**Response (Insufficient Stock):**
```json
{
  "success": false,
  "insufficient_stock": true,
  "current_version": 1,
  "current_quantity": 1,
  "message": "Insufficient stock available"
}
```

**Status Codes:**
- `200 OK`: Operation completed (success or failure)
- `400 Bad Request`: Invalid request body
- `500 Internal Server Error`: Database error

#### Health Check
```http
GET /health
```

**Description:** Service health status.

**Response:**
```json
{
  "status": "healthy",
  "service": "inventory-service-a"
}
```

---

## Service B (Store Service)

**Base URL:** `http://localhost:8081`

### Endpoints

#### Get Cached Item
```http
GET /store/inventory/{id}
```

**Description:** Retrieve a single inventory item from cache.

**Parameters:**
- `id` (path): Item ID (e.g., "SKU-123")

**Response:**
```json
{
  "item_id": "SKU-123",
  "name": "Wireless Headphones",
  "quantity": 50,
  "version": 1
}
```

**Status Codes:**
- `200 OK`: Item found in cache
- `404 Not Found`: Item not found in cache
- `500 Internal Server Error`: Service error

#### Get All Cached Items
```http
GET /store/inventory/all
```

**Description:** Retrieve all cached inventory items.

**Response:**
```json
{
  "items": [
    {
      "item_id": "SKU-123",
      "name": "Wireless Headphones",
      "quantity": 50,
      "version": 1
    }
  ]
}
```

**Status Codes:**
- `200 OK`: Success
- `500 Internal Server Error`: Service error

#### Store Checkout
```http
POST /store/checkout
```

**Description:** Initiate checkout with retry logic and exponential backoff.

**Request Body:**
```json
{
  "item_id": "SKU-123",
  "quantity": 2
}
```

**Response (Success):**
```json
{
  "success": true,
  "message": "Checkout successful"
}
```

**Response (Insufficient Stock):**
```json
{
  "success": false,
  "message": "Insufficient stock available",
  "error": "insufficient stock available"
}
```

**Response (Item Not Found):**
```json
{
  "success": false,
  "message": "Item not found",
  "error": "item not found"
}
```

**Response (Max Retries Exceeded):**
```json
{
  "success": false,
  "message": "Checkout failed after maximum retries",
  "error": "maximum retry attempts exceeded"
}
```

**Status Codes:**
- `200 OK`: Checkout successful
- `400 Bad Request`: Invalid request body
- `404 Not Found`: Item not found
- `409 Conflict`: Insufficient stock
- `503 Service Unavailable`: Max retries exceeded
- `500 Internal Server Error`: Service error

#### Health Check
```http
GET /health
```

**Description:** Service health status with cache information.

**Response:**
```json
{
  "status": "healthy",
  "service": "store-service-b",
  "cache_size": 5
}
```

---

## Error Handling

### Common Error Responses

#### Validation Error
```json
{
  "error": "Invalid JSON"
}
```

#### Service Unavailable
```json
{
  "error": "service temporarily unavailable"
}
```

### Error Codes

| Code | Description |
|------|-------------|
| `400` | Bad Request - Invalid input |
| `404` | Not Found - Resource not found |
| `409` | Conflict - Business logic conflict (e.g., insufficient stock) |
| `500` | Internal Server Error - System error |
| `503` | Service Unavailable - Temporary unavailability |

---

## Authentication

Currently, no authentication is implemented. In production, consider:
- API keys for service-to-service communication
- JWT tokens for user authentication
- Rate limiting to prevent abuse

---

## Rate Limiting

No rate limiting is currently implemented. Consider implementing:
- Per-IP rate limits
- Per-service rate limits
- Burst allowances for legitimate traffic spikes

---

## Monitoring & Observability

### Health Checks
Both services provide `/health` endpoints for monitoring.

### Logging
Services log:
- Checkout attempts and results
- Cache refresh operations
- Error conditions
- Performance metrics (latency, retry counts)

### Metrics (Future Enhancement)
Consider adding:
- Prometheus metrics export
- Request/response latency histograms
- Error rate counters
- Cache hit/miss ratios

