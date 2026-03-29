package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/ShubhamMor21/go-gateway/internal/config"
)

// Timeout attaches a deadline to the request context so that all downstream
// I/O operations (gRPC, Redis, Kafka, HTTP) automatically respect the budget.
//
// The previous goroutine-based implementation spawned a goroutine per request
// and had a subtle Fiber context reuse bug: if the timeout fired and the response
// was written, but the handler goroutine was still running, it would later
// attempt to use a *fiber.Ctx that had been returned to the sync.Pool and
// potentially reused by a different request — causing data races.
//
// This implementation is goroutine-free:
//   - It sets a deadline on c.UserContext() before calling c.Next().
//   - Pure CPU-bound handlers are not interrupted (acceptable for a gateway).
//   - All I/O operations that accept a context.Context are interrupted correctly.
func Timeout(cfg *config.Config) fiber.Handler {
	timeout := cfg.RequestTimeout

	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), timeout)
		defer cancel()

		c.SetUserContext(ctx)
		return c.Next()
	}
}
