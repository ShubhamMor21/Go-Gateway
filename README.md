# Go API Gateway

A production-ready API Gateway built with Go and Fiber. Acts as the single entry point for all client requests — handles auth, rate limiting, caching, and routes traffic to downstream services via gRPC.

---

## How it works

```
Client
  │
  ▼
┌─────────────────────────────────────────┐
│              API Gateway                │
│                                         │
│  RequestID → Security → CORS            │
│  → LoadShedding → Timeout → Logging     │
│  → Auth (JWT) → RateLimit (Redis)       │
│                                         │
│  GET /health       ──► Redis + gRPC check
│  GET /api/v1/users/:id ──► Redis cache  │
│                         or gRPC call    │
└─────────────────────────────────────────┘
         │                │
         ▼                ▼
      Redis            gRPC Service
   (cache + rate      (user data)
     limiting)
         │
         ▼
       Kafka
   (event producer,
    ready to wire)
```

---

## Stack

| Component      | Purpose                                                           |
| -------------- | ----------------------------------------------------------------- |
| **Go + Fiber** | HTTP server, 10K–100K concurrent requests                         |
| **Redis**      | Distributed rate limiting (sliding window) + response cache       |
| **gRPC**       | Calls downstream user service with circuit breaker + retry        |
| **Kafka**      | Event producer — wired and ready, attach to any handler           |
| **Zap**        | Structured JSON logging with request_id and user_id on every line |
| **Prometheus** | Request count, latency, error rate, circuit breaker state         |

---

## API Endpoints

### `GET /health`

No auth required. Used by load balancers.

**Response 200**

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "message": "Gateway is healthy",
    "components": {
      "redis": "healthy",
      "grpc": "closed"
    }
  }
}
```

**Response 503** — when Redis is down or circuit breaker is open.

---

### `GET /api/v1/users/:id`

Requires `Authorization: Bearer <jwt>`.

**Flow**

1. Validate JWT (HS256, checks expiry + issued-at)
2. Check Redis cache → return immediately on hit
3. On miss → call downstream gRPC service
4. Store result in Redis with TTL
5. Return user

**Response 200**

```json
{
  "success": true,
  "data": {
    "user_id": "abc-123",
    "name": "John Doe",
    "email": "john@example.com",
    "role": "user"
  }
}
```

---

## Error Response Format

All errors use the same envelope:

```json
{
  "success": false,
  "message": "Rate limit exceeded, please retry after a moment",
  "code": "RATE_LIMIT_EXCEEDED"
}
```

| HTTP | Code                       | When                                  |
| ---- | -------------------------- | ------------------------------------- |
| 401  | `UNAUTHORIZED`             | Missing or invalid JWT                |
| 401  | `TOKEN_EXPIRED`            | JWT has expired                       |
| 403  | `INSUFFICIENT_PERMISSIONS` | Wrong role                            |
| 429  | `RATE_LIMIT_EXCEEDED`      | Too many requests                     |
| 503  | `SERVICE_UNAVAILABLE`      | Load shedding or circuit breaker open |
| 504  | `REQUEST_TIMEOUT`          | Downstream took too long              |

---

## Middleware Stack

Applied to every request in this order:

```
1. Recover          — catches panics, no stack traces in production
2. RequestID        — generates X-Request-ID, validates if client sends one
3. Security         — CSP, HSTS, X-Frame-Options, removes Server header
4. CORS             — ENV-driven, no origin reflection with wildcard
5. LoadShedding     — 503 when in-flight requests exceed threshold
6. Timeout          — context deadline propagated to all I/O calls
7. Logging          — structured JSON: method, path, status, latency, request_id, user_id
8. Auth*            — JWT validation, injects user_id + role into context and headers
9. RateLimiter*     — Redis sliding window per-IP and per-user
```

`*` applied to `/api/v1` group only

---

## Running Locally

```bash
# 1. Copy config
cp .env.example .env

# 2. Fill in required values
#    JWT_SECRET=<at least 32 characters>
#    REDIS_URL=redis://localhost:6379

# 3. Start dependencies
docker run -d -p 6379:6379 redis:7-alpine

# 4. Run
go run ./cmd/server
```

**API port:** `:3030`
**Metrics port:** `:9090` → `GET /metrics` (Prometheus format)

---

## Key Environment Variables

| Variable                    | Required | Default | Description                                 |
| --------------------------- | -------- | ------- | ------------------------------------------- |
| `JWT_SECRET`                | ✅       | —       | Min 32 chars. Signs and verifies all tokens |
| `REDIS_URL`                 | ✅       | —       | `redis://host:port` or with auth            |
| `GRPC_SERVICE_URL`          | —        | —       | `host:port` of downstream user service      |
| `KAFKA_BROKERS`             | —        | —       | Comma-separated broker list                 |
| `RATE_LIMIT_REQUESTS`       | —        | `100`   | Max requests per window per IP/user         |
| `RATE_LIMIT_WINDOW_SECONDS` | —        | `60`    | Sliding window size                         |
| `CACHE_TTL_SECONDS`         | —        | `300`   | How long user responses are cached          |
| `LOG_LEVEL`                 | —        | `info`  | `debug` / `info` / `warn` / `error`         |
| `GRPC_TLS_ENABLED`          | —        | `false` | Enable TLS on gRPC connection               |
| `KAFKA_TLS_ENABLED`         | —        | `false` | Enable TLS on Kafka connection              |
| `KAFKA_SASL_ENABLED`        | —        | `false` | Enable SASL auth on Kafka                   |
| `METRICS_AUTH_TOKEN`        | —        | —       | Bearer token required on `/metrics`         |

See `.env.example` for the full list.

---

## Project Structure

```
cmd/server/         — entry point, wires everything together
internal/
  config/           — loads all config from ENV, validates on startup
  constants/        — all strings, header names, error codes (no inline strings anywhere)
  logger/           — Zap setup
  middleware/       — request_id, auth, rate_limiter, security, logging, timeout, load_shedding
  cache/            — Redis get/set with stampede protection (singleflight)
  grpc/             — singleton connection, circuit breaker, exponential-backoff retry
  queue/            — Kafka producer with TLS + SASL support
  handlers/         — health, users
  router/           — route registration
  metrics/          — Prometheus counters and histograms
  response/         — standard JSON envelope
pkg/proto/          — .proto source files (run go generate to compile)
```

---

## Docker

```bash
docker build -t go-gateway .
docker run -p 3030:3030 -p 9090:9090 --env-file .env go-gateway
```

Multi-stage build: Go builder → `distroless/static` (no shell, non-root, ~10 MB image).
