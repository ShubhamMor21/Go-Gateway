package handlers

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/circuitbreaker"
	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	grpcclient "github.com/ShubhamMor21/go-gateway/internal/grpc"
	"github.com/ShubhamMor21/go-gateway/internal/logger"
	"github.com/ShubhamMor21/go-gateway/internal/metrics"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// userIDPattern restricts path parameters to alphanumeric + hyphens (UUID-like).
// Rejects path traversal ("../"), null bytes, special chars that could poison
// Redis keys or be interpreted downstream.
var userIDPattern = regexp.MustCompile(`^[a-zA-Z0-9\-]{1,` + fmt.Sprintf("%d", constants.UserIDMaxLength) + `}$`)

// GetUser handles GET /users/:id.
// Flow:  cache hit → return early.
//
//	cache miss → gRPC call → cache result → return.
//
// Security: the :id path param is validated before being used as a Redis cache key.
func GetUser(cacheClient *cache.Client, grpcClient *grpcclient.Client, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		log := logger.FromContext(c.Locals(constants.LocalLogger))
		requestID, _ := c.Locals(constants.LocalRequestID).(string)
		callerUserID, _ := c.Locals(constants.LocalUserID).(string)

		targetUserID := c.Params("id")
		if targetUserID == "" || !userIDPattern.MatchString(targetUserID) {
			return response.Error(c,
				fiber.StatusBadRequest,
				constants.MsgBadRequest,
				constants.ErrCodeBadRequest,
			)
		}

		cacheKey := cache.CacheKey(callerUserID, fmt.Sprintf("/users/%s", targetUserID))

		// ── Cache hit ─────────────────────────────────────────────────
		var cached grpcclient.UserResponse
		if hit, err := cacheClient.Get(c.UserContext(), cacheKey, &cached); err == nil && hit {
			log.Debug(constants.MsgCacheHit, zap.String("key", cacheKey))
			metrics.CacheOperations.WithLabelValues("hit").Inc()
			return response.Success(c, fiber.StatusOK, cached)
		}

		metrics.CacheOperations.WithLabelValues("miss").Inc()
		log.Debug(constants.MsgCacheMiss, zap.String("key", cacheKey))

		// ── gRPC call ─────────────────────────────────────────────────
		user, err := grpcClient.GetUser(c.UserContext(), requestID, targetUserID)
		if err != nil {
			if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
				return response.Error(c,
					fiber.StatusServiceUnavailable,
					constants.MsgCircuitOpen,
					constants.ErrCodeCircuitOpen,
				)
			}
			log.Error(constants.MsgGRPCCallFailed,
				zap.String(constants.LocalRequestID, requestID),
				zap.Error(err),
			)
			return response.Error(c,
				fiber.StatusBadGateway,
				constants.MsgInternalError,
				constants.ErrCodeGRPCError,
			)
		}

		// ── Populate cache (best-effort) ──────────────────────────────
		_ = cacheClient.Set(c.UserContext(), cacheKey, user, time.Duration(cfg.CacheTTL))

		return response.Success(c, fiber.StatusOK, user)
	}
}
