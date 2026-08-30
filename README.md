# MiniEdge

**MiniEdge** is a lightweight, zero-third-party-dependency HTTP Edge Gateway and Reverse Proxy written in Go. It provides longest-prefix routing, connection-pooled HTTP/HTTPS proxying, active health monitoring, token-bucket rate limiting, controlled failure simulation, real-time observability telemetry, and a secure control REST API.

---

## Key Features

- **Zero External Dependencies**: Built entirely using Go standard library (`net/http`, `httputil`, `sync`, `crypto`, etc.).
- **Longest-Prefix Routing**: High-performance, deterministic prefix router with strict path boundary enforcement.
- **Reverse Proxy Engine**: Dynamic HTTP/HTTPS forwarding with connection pooling, header propagation (`X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host`), and timeout classification (`502 Bad Gateway`, `504 Gateway Timeout`).
- **Active Health Worker**: Periodic background probing (`UP`, `SLOW`, `DOWN` status classifications), failure count tracking, and per-service error isolation.
- **Token Bucket Rate Limiting**: Per-service traffic throttling returning HTTP `429 Too Many Requests` with `Retry-After` headers.
- **Failure Simulation**: Controlled fault injection supporting `NORMAL`, `FAIL` (HTTP `503 Service Unavailable`), and `DELAY` modes.
- **Observability Telemetry**: Bounded ring-buffer request logging (100 capacity) and point-in-time metrics aggregation (total/success/error counts, latency bounds, per-service status codes).
- **Secure Control REST API**: Admin endpoints for inspectable telemetry and runtime controls protected by constant-time API key verification, CORS headers, and 4 KB request body limits.
- **Graceful Shutdown**: Signal handling (`SIGINT`, `SIGTERM`) ensuring active requests drain and background workers stop cleanly.

---

## Architecture Overview

```text
                                +-----------------------------------+
                                |            Client Request         |
                                +-----------------------------------+
                                                  |
                                                  v
                                +-----------------------------------+
                                |       MiniEdge Gateway Server     |
                                |       (net/http Server + API)     |
                                +-----------------------------------+
                                                  |
                     +----------------------------+----------------------------+
                     |                                                         |
                     v                                                         v
          [ Control API Handler ]                                   [ Proxy Router Flow ]
      (/api/logs, /api/metrics, etc.)                                 (r.URL.Path Match)
                     |                                                         |
                     v                                                         v
        +-------------------------+                               +--------------------------+
        |  Auth / CORS / Payload  |                               |    Token Bucket Rate     |
        |  Verification Checks    |                               |    Limiting Evaluation   |
        +-------------------------+                               +--------------------------+
                                                                               |
                                                                               v
                                                                  +--------------------------+
                                                                  |    Controlled Failure    |
                                                                  |    Simulation Check      |
                                                                  +--------------------------+
                                                                               |
                                                                               v
                                                                  +--------------------------+
                                                                  |   Reverse Proxy Engine   |
                                                                  | (httputil.ReverseProxy)  |
                                                                  +--------------------------+
                                                                               |
                                                                               v
                                                                  +--------------------------+
                                                                  |     Upstream Service     |
                                                                  +--------------------------+
```

### Component Breakdown

1. **Config (`internal/config`)**: Parses JSON configuration files and validates service IDs, URLs, schemes, listen addresses, and route boundaries.
2. **Router (`internal/router`)**: Pre-sorts routes by length descending to enforce deterministic longest-prefix matching.
3. **Service Registry (`internal/router`)**: Thread-safe in-memory mapping of Service IDs to Service definitions.
4. **Gateway Handler (`internal/gateway`)**: Orchestrates request processing, generates unique `X-Request-ID` headers (`req_<hex>`), invokes rate limiters and failure simulators, and publishes events to observability.
5. **Reverse Proxy (`internal/proxy`)**: Wraps `httputil.ReverseProxy` with dynamic connection pooling, subpath joining, query string preservation, and `X-Forwarded-*` header forwarding.
6. **Health Checker & Worker (`internal/health`)**: Runs periodic background probes against configured service `/health` endpoints and updates shared health state.
7. **Observability Store (`internal/observability`)**: Thread-safe ring buffer storing recent request events and calculating global & per-service cumulative metrics.
8. **Rate Limiter Store (`internal/ratelimit`)**: Thread-safe per-service token buckets for request throttling.
9. **Simulation Store (`internal/simulation`)**: Maintains failure simulation modes (`NORMAL`, `FAIL`, `DELAY`) per service.
10. **Control API (`internal/api`)**: REST API handlers for telemetry retrieval and administrative runtime configuration.
11. **Application Bootstrap (`cmd/main.go`)**: Composition root handling CLI flags, environment variables, dependency wiring, worker lifecycle, and graceful server shutdown.

---

## Repository Structure

```text
MiniEdge-GitHub/
├── cmd/
│   └── main.go                 # Application entrypoint and composition root
├── config.example.json         # Validated sample configuration file
├── go.mod                      # Go 1.24 module manifest (zero external dependencies)
├── internal/
│   ├── api/                    # Control REST API handlers, DTOs, security & unit tests
│   ├── config/                 # Configuration JSON parser, validator & tests
│   ├── gateway/                # Core HTTP gateway handler, request ID generator & tests
│   ├── health/                 # Health check worker, probe runner, state store & tests
│   ├── integration/            # End-to-end integration & pipeline tests
│   ├── model/                  # Data models, interfaces, and error definitions
│   ├── observability/          # Bounded ring buffer log store and metrics accumulator & tests
│   ├── proxy/                  # Reverse proxy wrapper, connection pool, subpath joiner & tests
│   ├── ratelimit/              # Token bucket algorithm, rate limiter store & tests
│   ├── router/                 # Longest-prefix router, static service registry & tests
│   └── simulation/             # Controlled failure simulator store & tests
└── README.md                   # Project documentation
```

---

## Requirements

- **Go**: Version `1.24` or higher.
- **Dependencies**: None (Go Standard Library only).

---

## Configuration

MiniEdge is configured via a JSON file. Refer to [`config.example.json`](file:///c:/Users/Henil%20Patel/MiniEdge-Backend/MiniEdge-GitHub/config.example.json) for a complete example.

```json
{
  "listen_addr": "127.0.0.1:8080",
  "services": [
    {
      "id": "users-service",
      "name": "User Management Service",
      "upstream": "http://127.0.0.1:8081",
      "health_path": "/health"
    },
    {
      "id": "admin-service",
      "name": "Admin Portal Service",
      "upstream": "http://127.0.0.1:8083",
      "health_path": "/status"
    }
  ],
  "routes": [
    {
      "path": "/users/",
      "service": "users-service"
    },
    {
      "path": "/users/admin/",
      "service": "admin-service"
    }
  ]
}
```

### Configuration Fields

- `listen_addr`: IP and port address on which MiniEdge listens (e.g. `127.0.0.1:8080`).
- `services`: List of backend services:
  - `id`: Unique identifier string for the service.
  - `name`: Human-readable service name.
  - `upstream`: Base HTTP/HTTPS URL of the target upstream backend (e.g. `http://127.0.0.1:8081` or `http://localhost:8080/v1`).
  - `health_path`: Relative URL path probed by the health checker (e.g. `/health`).
- `routes`: Path prefix mappings:
  - `path`: Path prefix starting with `/`. Longest matching prefix wins (e.g. `/users/admin/` takes precedence over `/users/`).
  - `service`: Service ID referenced by this route.

---

## Running MiniEdge

### Commands

To build the executable:
```bash
go build -o miniedge ./cmd/main.go
```

To run MiniEdge with a custom configuration file:
```bash
./miniedge -config ./config.json
```

Or run directly with `go run`:
```bash
go run ./cmd/main.go -config ./config.json
```

### Flag & Fallback Behavior

- `-config <path>`: Specifies path to configuration JSON file (defaults to `config.json`).
- **Fallback**: If `config.json` does not exist in the working directory when `-config` is left at its default value, MiniEdge automatically attempts to load [`config.example.json`](file:///c:/Users/Henil%20Patel/MiniEdge-Backend/MiniEdge-GitHub/config.example.json).

### Environment Variables

- `MINIEDGE_API_KEY`: Secret string required for administrative POST control endpoints (`X-API-Key` header). If unset, POST mutation requests are rejected with HTTP 401.
- `MINIEDGE_ALLOWED_ORIGIN`: Value for `Access-Control-Allow-Origin` CORS response headers (defaults to `http://localhost:3000`).

---

## Control REST API Reference

MiniEdge exposes administrative and telemetry endpoints under the `/api/` prefix.

| Endpoint | Method | Authentication | Description |
| :--- | :---: | :---: | :--- |
| `/api/logs` | `GET` | None | Retrieves up to `limit` recent request logs (optional `?limit=50&service=users-service`). |
| `/api/metrics` | `GET` | None | Retrieves cumulative global and per-service metrics (request counts, latency bounds, status codes). |
| `/api/health` | `GET` | None | Returns point-in-time active health state (`UP`, `SLOW`, `DOWN`) for all services. |
| `/api/simulations` | `GET` | None | Returns active failure simulation modes (`NORMAL`, `FAIL`, `DELAY`) for all services. |
| `/api/simulations` | `POST` | `X-API-Key` | Updates simulation mode and delay for a service. |
| `/api/ratelimits` | `GET` | None | Returns rate-limiting token bucket configurations for all services. |
| `/api/ratelimits` | `POST` | `X-API-Key` | Updates rate limit (`requests_per_second`, `burst`, `enabled`) for a service. |
| `/api/*` | `OPTIONS` | None | Returns CORS preflight response (`204 No Content`). |

---

## Authentication & Security

1. **API Key Authentication**:
   - Administrative POST endpoints require `X-API-Key: <key>`.
   - Verified using `crypto/subtle.ConstantTimeCompare` to prevent timing attacks.
   - If `MINIEDGE_API_KEY` is not set, administrative POST endpoints reject requests with HTTP 401 Unauthorized.
   - Secret keys are never logged or exposed in HTTP responses.
2. **CORS Support**:
   - Sets `Access-Control-Allow-Origin`, `Access-Control-Allow-Methods`, `Access-Control-Allow-Headers`, and `Access-Control-Max-Age`.
   - Handles `OPTIONS` preflight requests.
3. **Payload Limits**:
   - `POST` control endpoints enforce a 4 KB body limit via `http.MaxBytesReader`. Payloads exceeding 4 KB return `HTTP 413 Payload Too Large`.
4. **TLS Termination Limitation**:
   - MiniEdge operates as an HTTP server. For HTTPS in production, place a TLS-terminating load balancer or reverse proxy in front of MiniEdge.

---

## Health Monitoring

The background health worker (`internal/health`) periodically probes each service's `health_path`:

- **UP**: Service responds with HTTP 200–399 within slow threshold.
- **SLOW**: Service responds with HTTP 200–399, but latency exceeds `slowThreshold` (default: 500 ms).
- **DOWN**: Service connection fails, times out (default: 2 s), or returns HTTP 400–599.
- **Failure Tracking**: Consecutive failure counts increment on `DOWN` and reset to 0 upon recovery (`UP`/`SLOW`).

---

## Rate Limiting

Per-service rate limiting uses a thread-safe **Token Bucket** algorithm (`internal/ratelimit`):

- Configurable `requests_per_second` (refill rate) and `burst` (capacity).
- Requests exceeding bucket capacity return `HTTP 429 Too Many Requests` with a `Retry-After` header indicating estimated wait time in seconds.
- Can be dynamically enabled or disabled per service via `POST /api/ratelimits`.

---

## Failure Simulation

Controlled fault injection (`internal/simulation`) allows chaos testing without affecting actual upstreams:

- **NORMAL**: Requests are forwarded normally.
- **FAIL**: Requests immediately return `HTTP 503 Service Unavailable` (`"simulation_active"` error) without contacting upstream.
- **DELAY**: Requests are delayed by `delay_ms` before forwarding. Supports request context cancellation.

---

## Observability & Telemetry

- **Bounded Ring Buffer**: Retains the last 100 request events in memory.
- **Cumulative Metrics**: Tracks total, successful, and error request counts, minimum/maximum/average latencies, and status code distributions per service.
- **In-Memory Telemetry**: Metrics and logs are stored in memory and reset upon process restart.

---

## Testing & Verification

Run the complete test suite, linter, build check, and diff check:

```bash
# Run all unit and E2E integration tests (uncached)
go test -count=1 ./...

# Run static code analysis
go vet ./...

# Verify clean compilation
go build ./cmd/...

# Check for formatting/whitespace issues
git diff --check
```

---

## Example Usage

### 1. Query Telemetry Endpoints (Unauthenticated)

```bash
# Check service health status
curl -s http://127.0.0.1:8080/api/health | jq .

# Get cumulative metrics
curl -s http://127.0.0.1:8080/api/metrics | jq .

# Get recent request logs
curl -s "http://127.0.0.1:8080/api/logs?limit=10" | jq .
```

### 2. Configure Failure Simulation (Authenticated)

```bash
# Inject 500ms delay into users-service
curl -s -X POST http://127.0.0.1:8080/api/simulations \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-api-key" \
  -d '{"service_id": "users-service", "mode": "DELAY", "delay_ms": 500}' | jq .

# Reset users-service to normal mode
curl -s -X POST http://127.0.0.1:8080/api/simulations \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-api-key" \
  -d '{"service_id": "users-service", "mode": "NORMAL", "delay_ms": 0}' | jq .
```

### 3. Update Rate Limits (Authenticated)

```bash
# Set rate limit to 50 req/sec with burst of 100
curl -s -X POST http://127.0.0.1:8080/api/ratelimits \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-api-key" \
  -d '{"service_id": "users-service", "requests_per_second": 50, "burst": 100, "enabled": true}' | jq .
```

---

## Known Limitations

1. **In-Memory Telemetry**: Request logs and metrics are stored in memory and reset upon restart.
2. **Static Service Management**: Services and routes are defined at startup via JSON configuration; dynamic runtime service registration APIs are not implemented.
3. **HTTP / TLS**: MiniEdge listens as an HTTP server. TLS termination must be handled by an upstream reverse proxy or edge load balancer.