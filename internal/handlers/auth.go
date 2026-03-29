package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/logger"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// Logout adds the current token to the revocation store so it cannot be reused
// even if it has not yet expired.
//
// How it works:
//  1. Auth middleware computed SHA-256(raw_token) and stored it in LocalTokenHash.
//  2. Auth middleware stored the token's expiry time in LocalTokenExp.
//  3. This handler reads both values, then stores the hash in Redis with a TTL
//     equal to the remaining lifetime of the token.
//  4. From this moment, Auth middleware will reject any request bearing this token.
//
// No request body is needed — the token is already authenticated upstream.
func Logout(cacheClient *cache.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		log := logger.FromContext(c.Locals(constants.LocalLogger))
		requestID, _ := c.Locals(constants.LocalRequestID).(string)

		tokenHash, _ := c.Locals(constants.LocalTokenHash).(string)
		if tokenHash == "" {
			// Should not happen — Auth middleware always sets this.
			return response.Error(c,
				fiber.StatusInternalServerError,
				constants.MsgInternalError,
				constants.ErrCodeInternalError,
			)
		}

		// Calculate TTL = how long until the token would have expired naturally.
		ttl := time.Duration(0)
		if exp, ok := c.Locals(constants.LocalTokenExp).(time.Time); ok {
			ttl = time.Until(exp)
		}

		if err := cacheClient.RevokeToken(c.UserContext(), tokenHash, ttl); err != nil {
			// Log but do not surface internals to the caller.
			log.Error("logout: failed to revoke token in Redis",
				zap.String(constants.LocalRequestID, requestID),
				zap.Error(err),
			)
			// Still return success — the token will expire naturally.
			// Failing logout is worse UX than a brief window where the token remains valid.
		}

		log.Info("token revoked",
			zap.String(constants.LocalRequestID, requestID),
			zap.String(constants.LocalUserID, func() string {
				s, _ := c.Locals(constants.LocalUserID).(string)
				return s
			}()),
		)

		return response.Success(c, fiber.StatusOK, fiber.Map{
			"message": constants.MsgLogoutSuccess,
		})
	}
}
