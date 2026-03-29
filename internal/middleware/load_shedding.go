package middleware

import (
	"sync/atomic"

	"github.com/gofiber/fiber/v2"

	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// LoadShedding rejects new requests when the number of in-flight requests
// exceeds cfg.LoadShedMaxConnections. This protects the gateway process itself
// from memory exhaustion and protects downstream services from being overwhelmed.
//
// Fix: uses a compare-and-swap loop to atomically check AND increment the counter.
// The previous AddInt64-then-compare approach allowed up to (max + concurrency)
// goroutines to pass the check simultaneously under high load.
func LoadShedding(cfg *config.Config) fiber.Handler {
	var active int64
	max := int64(cfg.LoadShedMaxConnections)

	return func(c *fiber.Ctx) error {
		// Atomically reserve a slot only if we are under the limit.
		// CAS loop: read → check → swap. Retries only on contention, never busy-waits
		// indefinitely because the counter only moves ±1 per goroutine.
		for {
			current := atomic.LoadInt64(&active)
			if current >= max {
				return response.Error(c,
					fiber.StatusServiceUnavailable,
					constants.MsgServiceUnavailable,
					constants.ErrCodeServiceUnavailable,
				)
			}
			if atomic.CompareAndSwapInt64(&active, current, current+1) {
				break
			}
			// Another goroutine changed the counter between our Load and CAS — retry.
		}

		defer atomic.AddInt64(&active, -1)
		return c.Next()
	}
}
