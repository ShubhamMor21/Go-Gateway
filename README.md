# Go API Gateway

A production-ready API Gateway built with Go and Fiber. Acts as the single entry point for all client requests — handles auth, rate limiting, token revocation, IP blocklisting, RBAC, and routes traffic to downstream services via gRPC.

---

## How it works

```
Client
  │
  ▼
┌─────────────────────────────────────────────┐
│               API Gateway                   │
│                                             │
│  Recover → RequestID → IPBlocklist          │
│  → Security → CORS → LoadShedding           │
│  → Timeout → RateLimit (Redis, global)      │
│  → Logging → Auth (JWT + revocation check)  │
│                                             │
│  GET  /health              ──► Redis + gRPC │
│  POST /api/v1/auth/logout  ──► revoke token │
│  GET  /api/v1/users/:id    ──► cache/gRPC   │
│  GET  /api/v1/admin/status ──► admin only   │
└─────────────────────────────────────────────┘
         │                │
         ▼                ▼
      Redis            gRPC Service
  (cache + rate       (user data)
   limit + token
   revocation +
   IP blocklist)
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
| **Redis**      | Rate limiting, response cache, token revocation, IP blocklist     |
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

### `POST /api/v1/auth/logout`

Requires `Authorization: Bearer <jwt>`. Revokes the current token so it cannot be reused even before expiry.

**How it works:** The token's SHA-256 hash is stored in Redis with a TTL equal to the token's remaining lifetime. All subsequent requests with this token are rejected with 401.

**Response 200**

```json
{
  "success": true,
  "data": {
    "message": "Logged out successfully"
  }
}
```

---

### `GET /api/v1/users/:id`

Requires `Authorization: Bearer <jwt>`.

**Ownership check:** Regular users can only fetch their own profile. Admins can fetch any.

**Flow**

1. Validate JWT (HS256/RS256/ES256, checks expiry + issued-at)
2. Check token revocation in Redis
3. Ownership check (BOLA prevention)
4. Check Redis cache → return immediately on hit
5. On miss → call downstream gRPC service
6. Store result in Redis with TTL
7. Return user

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

**Response 403** — when a regular user requests another user's profile.

---

### `GET /api/v1/admin/status`

Requires `Authorization: Bearer <jwt>` with `role: admin`. Returns 403 for any other role.

**Response 200**

```json
{
  "status": "ok",
  "message": "admin access confirmed"
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

| HTTP | Code                       | When                                          |
| ---- | -------------------------- | --------------------------------------------- |
| 401  | `UNAUTHORIZED`             | Missing or invalid JWT                        |
| 401  | `TOKEN_EXPIRED`            | JWT has expired                               |
| 401  | `TOKEN_REVOKED`            | Token was explicitly revoked via logout       |
| 403  | `FORBIDDEN_OWNERSHIP`      | User requested another user's resource        |
| 403  | `FORBIDDEN_ROLE`           | Insufficient role for this endpoint           |
| 403  | `IP_BLOCKED`               | Caller IP is on the blocklist                 |
| 429  | `RATE_LIMIT_EXCEEDED`      | Too many requests                             |
| 503  | `SERVICE_UNAVAILABLE`      | Load shedding or circuit breaker open         |
| 504  | `REQUEST_TIMEOUT`          | Downstream took too long                      |

---

## Middleware Stack

Applied to every request in this order:

```
1. Recover       — catches panics, stack traces only in debug mode
2. RequestID     — generates X-Request-ID, validates if client sends one
3. IPBlocklist   — rejects blocked IPs before any further processing
4. Security      — CSP, HSTS, X-Frame-Options, removes Server header
5. CORS          — ENV-driven, no origin reflection with wildcard
6. LoadShedding  — 503 when in-flight requests exceed threshold
7. Timeout       — context deadline propagated to all I/O calls
8. RateLimiter   — Redis sliding window, global (covers unauthenticated requests too)
9. Logging       — structured JSON: method, path, status, latency, request_id, user_id
── /api/v1 group ──
10. Auth         — JWT validation + revocation check, injects user_id + role into context
```

Rate limiting runs **before** auth so brute-force probes and unauthenticated requests are throttled too.

---

## Security Features

| Feature               | Implementation                                                                           |
| --------------------- | ---------------------------------------------------------------------------------------- |
| JWT algorithms        | HS256 (default), RS256, ES256 — configured via `JWT_ALGORITHM`                           |
| Token revocation      | SHA-256(token) stored in Redis on logout; checked on every authenticated request         |
| RBAC                  | `RequireRole("admin")` middleware; O(1) map lookup; returns 403 for insufficient role    |
| BOLA prevention       | Ownership check in `GET /users/:id` — regular users blocked from other users' data      |
| IP blocklist          | Redis SET `blocked_ips`; manage with `SADD`/`SREM`; first check in middleware chain     |
| Rate limiting         | Sliding window Lua script with UUID nonce; fail-closed by default (`RATE_LIMIT_FAIL_OPEN=false`) |
| Load shedding         | CAS atomic counter; 503 when in-flight requests exceed `LOAD_SHED_MAX_CONNECTIONS`      |
| Metrics auth          | Optional Bearer token on `/metrics` via `METRICS_AUTH_TOKEN`                            |
| Redis TLS enforcement | `REDIS_REQUIRE_TLS=true` blocks startup if `redis://` (plaintext) is configured         |
| gRPC TLS              | Defaults to `true`; TLS 1.3 minimum; `GRPC_SERVER_NAME_OVERRIDE` for staging certs      |
| Security headers      | HSTS, CSP, X-Frame-Options, X-Content-Type-Options on every response                   |
| Startup warnings      | Logs `WARN` when metrics token unset, Redis plaintext, or gRPC TLS disabled             |

Covers all **OWASP API Security Top 10 (2023)** items.

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

| Variable                    | Required | Default  | Description                                              |
| --------------------------- | -------- | -------- | -------------------------------------------------------- |
| `JWT_SECRET`                | HS256 ✅ | —        | Min 32 chars. Signs and verifies HS256 tokens            |
| `JWT_ALGORITHM`             | —        | `HS256`  | `HS256` \| `RS256` \| `ES256`                            |
| `JWT_PUBLIC_KEY_PATH`       | RS/ES ✅ | —        | Path to PEM public key file (RS256/ES256)                |
| `JWT_PUBLIC_KEY`            | RS/ES ✅ | —        | Inline PEM public key (alternative to path)              |
| `REDIS_URL`                 | ✅       | —        | `redis://host:port` or `rediss://` for TLS               |
| `REDIS_REQUIRE_TLS`         | —        | `false`  | Fail startup if Redis URL is not `rediss://`             |
| `IP_BLOCKLIST_ENABLED`      | —        | `true`   | Check every request IP against Redis `blocked_ips` SET  |
| `GRPC_SERVICE_URL`          | —        | —        | `host:port` of downstream user service                   |
| `GRPC_TLS_ENABLED`          | —        | `true`   | Enable TLS on gRPC connection (TLS 1.3 minimum)          |
| `KAFKA_BROKERS`             | —        | —        | Comma-separated broker list                              |
| `RATE_LIMIT_REQUESTS`       | —        | `100`    | Max requests per window per IP/user                      |
| `RATE_LIMIT_WINDOW_SECONDS` | —        | `60`     | Sliding window size                                      |
| `RATE_LIMIT_FAIL_OPEN`      | —        | `false`  | `true` = allow all when Redis is down (not recommended)  |
| `CACHE_TTL_SECONDS`         | —        | `300`    | How long user responses are cached                       |
| `METRICS_AUTH_TOKEN`        | —        | —        | Bearer token required on `/metrics`                      |
| `LOG_LEVEL`                 | —        | `info`   | `debug` / `info` / `warn` / `error`                      |

See `.env.example` for the full list with all options.

---

## Managing the IP Blocklist

The blocklist is stored in a Redis SET named `blocked_ips`. Manage it directly:

```bash
# Block an IP
redis-cli SADD blocked_ips 1.2.3.4

# Unblock an IP
redis-cli SREM blocked_ips 1.2.3.4

# View all blocked IPs
redis-cli SMEMBERS blocked_ips
```

---

## Project Structure

```
cmd/server/         — entry point, wires everything together
internal/
  config/           — loads all config from ENV, validates on startup
  constants/        — all strings, header names, error codes (no inline strings anywhere)
  logger/           — Zap setup
  middleware/       — request_id, ip_blocklist, auth, rate_limiter, security,
                      logging, timeout, load_shedding
  cache/            — Redis get/set, stampede protection, token revocation, IP blocklist
  grpc/             — singleton connection, circuit breaker, exponential-backoff retry
  queue/            — Kafka producer with TLS + SASL support
  handlers/         — health, users, auth (logout)
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
