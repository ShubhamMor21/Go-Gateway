package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/logger"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// IPBlocklist checks every inbound request IP against a Redis SET.
// It is placed first in the global middleware chain so blocked IPs are
// rejected before any other processing (JWT parsing, rate-limit Lua, etc.)
// saving CPU and preventing enumeration of internal error messages.
//
// Managing the blocklist:
//   Add:    SADD blocked_ips 1.2.3.4
//   Remove: SREM blocked_ips 1.2.3.4
//   List:   SMEMBERS blocked_ips
//
// Or use the cache.Client methods BlockIP / UnblockIP from an admin handler.
//
// Fail strategy: fail-open on Redis errors. An IP blocklist is a defence-in-depth
// control — its absence (during Redis downtime) is acceptable as a short-term
// degradation. The rate limiter and load shedder remain active as fallbacks.
func IPBlocklist(cacheClient *cache.Client, enabled bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !enabled {
			return c.Next()
		}

		log := logger.FromContext(c.Locals(constants.LocalLogger))
		ip := c.IP()

		blocked, err := cacheClient.IsIPBlocked(context.Background(), ip)
		if err != nil {
			// Fail-open: log the Redis error but don't block traffic.
			log.Warn("ip_blocklist: Redis check failed, failing open",
				zap.String("ip", ip),
				zap.Error(err),
			)
			return c.Next()
		}

		if blocked {
			log.Warn("ip_blocklist: blocked IP rejected",
				zap.String("ip", ip),
				zap.String(constants.LocalRequestID, func() string {
					s, _ := c.Locals(constants.LocalRequestID).(string)
					return s
				}()),
			)
			return response.Error(c,
				fiber.StatusForbidden,
				constants.MsgIPBlocked,
				constants.ErrCodeIPBlocked,
			)
		}

		return c.Next()
	}
}
