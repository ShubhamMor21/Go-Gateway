package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// slidingWindowScript is a Redis Lua script that implements a sliding-window rate limiter.
// It is atomic — no race conditions even under high concurrency.
//
// KEYS[1]  – rate-limit key (e.g. "rl:ip:1.2.3.4")
// ARGV[1]  – current Unix timestamp in milliseconds
// ARGV[2]  – window size in milliseconds
// ARGV[3]  – maximum allowed requests per window
// ARGV[4]  – unique nonce (UUID) — guarantees member uniqueness even when two
//             requests share the same millisecond timestamp (common in tests and
//             burst traffic). The previous math.random approach caused member
//             collisions: same-ms requests seeded the same random value → ZADD
//             overwrote the existing member → ZCARD did not increase → rate limit
//             was not enforced.
//
// Returns 1 if the request is allowed, 0 if it should be rejected.
var slidingWindowScript = redis.NewScript(`
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local nonce  = ARGV[4]
local cutoff = now - window

redis.call('ZREMRANGEBYSCORE', key, 0, cutoff)
local count = redis.call('ZCARD', key)

if count >= limit then
  return 0
end

local member = tostring(now) .. ':' .. nonce
redis.call('ZADD', key, now, member)
redis.call('PEXPIRE', key, window + 1000)
return 1
`)

// localFallbackLimiter provides per-key token-bucket rate limiting using
// in-process memory when Redis is unavailable. This is NOT distributed —
// it only limits requests hitting this specific process instance.
// It is the last line of defence; Redis downtime should be alerted immediately.
type localFallbackLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func newLocalFallback(requestsPerWindow int, windowSeconds int) *localFallbackLimiter {
	r := rate.Limit(float64(requestsPerWindow) / float64(windowSeconds))
	return &localFallbackLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        r,
		b:        requestsPerWindow,
	}
}

func (l *localFallbackLimiter) allow(key string) bool {
	l.mu.Lock()
	lim, ok := l.limiters[key]
	if !ok {
		lim = rate.NewLimiter(l.r, l.b)
		l.limiters[key] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}

// RateLimiter implements per-IP and per-user distributed rate limiting using
// Redis sorted sets with a sliding-window algorithm.
//
// Fail strategy (controlled by cfg.RateLimitFailOpen):
//   - false (default, recommended for fintech): use local in-process token-bucket
//     as a fallback when Redis is unreachable. Distributed enforcement degrades to
//     per-instance enforcement — still protects against abuse, alerts on Redis outage.
//   - true: fail open (allow all requests). Use only where availability is paramount.
//
// All limits come from cfg (ENV); no inline numbers.
func RateLimiter(cfg *config.Config, rdb *redis.Client) fiber.Handler {
	window := time.Duration(cfg.RateLimitWindowSeconds) * time.Second
	windowMs := window.Milliseconds()
	limit := cfg.RateLimitRequests
	failOpen := cfg.RateLimitFailOpen

	local := newLocalFallback(limit, cfg.RateLimitWindowSeconds)

	return func(c *fiber.Ctx) error {
		ctx := context.Background()
		nowMs := time.Now().UnixMilli()

		// ── Per-IP limit ──────────────────────────────────────────────
		ipKey := fmt.Sprintf("rl:ip:%s", c.IP())
		allowed, redisErr := isAllowed(ctx, rdb, ipKey, nowMs, windowMs, int64(limit))

		if redisErr != nil {
			if failOpen {
				return c.Next()
			}
			// Fallback: enforce locally so Redis outage doesn't open the flood gate.
			if !local.allow(ipKey) {
				return response.Error(c,
					fiber.StatusTooManyRequests,
					constants.MsgRateLimitExceeded,
					constants.ErrCodeRateLimitExceeded,
				)
			}
			return c.Next()
		}

		if !allowed {
			return response.Error(c,
				fiber.StatusTooManyRequests,
				constants.MsgRateLimitExceeded,
				constants.ErrCodeRateLimitExceeded,
			)
		}

		// ── Per-user limit (authenticated requests only) ──────────────
		if userID, ok := c.Locals(constants.LocalUserID).(string); ok && userID != "" {
			userKey := fmt.Sprintf("rl:user:%s", userID)
			allowed, redisErr = isAllowed(ctx, rdb, userKey, nowMs, windowMs, int64(limit))

			if redisErr != nil {
				if failOpen {
					return c.Next()
				}
				if !local.allow(userKey) {
					return response.Error(c,
						fiber.StatusTooManyRequests,
						constants.MsgRateLimitExceeded,
						constants.ErrCodeRateLimitExceeded,
					)
				}
				return c.Next()
			}

			if !allowed {
				return response.Error(c,
					fiber.StatusTooManyRequests,
					constants.MsgRateLimitExceeded,
					constants.ErrCodeRateLimitExceeded,
				)
			}
		}

		return c.Next()
	}
}

func isAllowed(ctx context.Context, rdb *redis.Client, key string, nowMs, windowMs, limit int64) (bool, error) {
	result, err := slidingWindowScript.Run(ctx, rdb,
		[]string{key},
		nowMs, windowMs, limit, uuid.New().String(),
	).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
