# MiniEdge — Lightweight Edge Gateway & Operations Center

**MiniEdge** is a lightweight, zero-third-party-dependency HTTP Edge Gateway, Reverse Proxy, and Operations Dashboard written in Go and Next.js / Vanilla Web. It provides longest-prefix routing, connection-pooled HTTP/HTTPS proxying, active health monitoring, token-bucket rate limiting, controlled failure simulation, real-time observability telemetry, a secure control REST API, and a real-time Developer Operations Center dashboard.

---

## Architecture Overview

```text
                                +-----------------------------------+
                                |            Client / UI            |
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
7. **Observability Store (`internal/observability` & `internal/metrics`)**: Thread-safe ring buffer storing recent request events and calculating global & per-service cumulative metrics.
8. **Rate Limiter Store (`internal/ratelimit`)**: Thread-safe per-service token buckets for request throttling.
9. **Simulation Store (`internal/simulation`)**: Maintains failure simulation modes (`NORMAL`, `FAIL`, `DELAY`) per service.
10. **Control API (`internal/api`)**: REST API handlers for telemetry retrieval and administrative runtime configuration.
11. **Application Bootstrap (`cmd/main.go`)**: Composition root handling CLI flags, environment variables, dependency wiring, worker lifecycle, and graceful server shutdown.
12. **Frontend Dashboard (`app/`, `components/`, `lib/`)**: Real-time Developer Operations Center built with Next.js & Vanilla Web components.

---

## Backend Requirements & Running

- **Go**: Version `1.24` or higher.
- **Dependencies**: None (Go Standard Library only).

### Commands

To build the backend executable:
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

### Environment Variables

- `MINIEDGE_API_KEY`: Secret string required for administrative POST control endpoints (`X-API-Key` header). If unset, POST mutation requests are rejected with HTTP 401.
- `MINIEDGE_ALLOWED_ORIGIN`: Value for `Access-Control-Allow-Origin` CORS response headers (defaults to `*` or deployed Vercel URL).
- `PORT`: Port number provided dynamically by host platforms (e.g. Render).

---

## Control REST API Reference

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

## Frontend Operations Center

The frontend provides an interactive monitoring and control dashboard for microservice health, traffic metrics, request logs, and chaos simulations.

### Running the Frontend locally

```bash
npm install
npm run dev
```

For production build:
```bash
npm run build
npm start
```

### Frontend Integration Contract

Update `lib/client.js` or set environment variable `NEXT_PUBLIC_API_BASE_URL`:

```javascript
import { apiClient } from './lib/client.js';

// Switch from mock mode to live backend
apiClient.setMode('real');
apiClient.setBaseUrl('http://localhost:8080');
```
