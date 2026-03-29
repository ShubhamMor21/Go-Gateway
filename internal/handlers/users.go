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

var userIDPattern = regexp.MustCompile(`^[a-zA-Z0-9\-]{1,` + fmt.Sprintf("%d", constants.UserIDMaxLength) + `}$`)

// GetUser handles GET /api/v1/users/:id
//
// [CRITICAL] Ownership check:
//   A regular user may only fetch their own profile.
//   An admin (role == "admin") may fetch any profile.
//   Without this check, any authenticated user could enumerate all user data
//   by guessing IDs — a classic Broken Object Level Authorization (OWASP API1).
func GetUser(cacheClient *cache.Client, grpcClient *grpcclient.Client, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		log := logger.FromContext(c.Locals(constants.LocalLogger))
		requestID, _ := c.Locals(constants.LocalRequestID).(string)
		callerUserID, _ := c.Locals(constants.LocalUserID).(string)
		callerRole, _ := c.Locals(constants.LocalUserRole).(string)

		targetUserID := c.Params("id")
		if targetUserID == "" || !userIDPattern.MatchString(targetUserID) {
			return response.Error(c,
				fiber.StatusBadRequest,
				constants.MsgBadRequest,
				constants.ErrCodeBadRequest,
			)
		}

		// ── [CRITICAL] Ownership / RBAC gate ─────────────────────────
		// A user may only read their own record. Admins bypass this check.
		// This prevents horizontal privilege escalation: user A reading user B's data.
		if callerUserID != targetUserID && callerRole != "admin" {
			log.Warn("ownership violation attempt",
				zap.String(constants.LocalRequestID, requestID),
				zap.String("caller_id", callerUserID),
				zap.String("target_id", targetUserID),
			)
			return response.Error(c,
				fiber.StatusForbidden,
				constants.MsgForbiddenOwnership,
				constants.ErrCodeForbiddenOwnership,
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

		_ = cacheClient.Set(c.UserContext(), cacheKey, user, time.Duration(cfg.CacheTTL))

		return response.Success(c, fiber.StatusOK, user)
	}
}
