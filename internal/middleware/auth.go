package middleware

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gofiber/fiber/v2"

	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// jwtClaims is the canonical set of claims expected in every access token.
type jwtClaims struct {
	UserID string `json:"sub"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Auth validates the JWT Bearer token and injects user_id + role into Fiber Locals
// and downstream request headers. The JWT secret comes exclusively from cfg (ENV).
//
// Security hardening applied:
//   - Algorithm pinned to HS256 — "none" and RS/ES variants are rejected at parse time
//     via jwt.WithValidMethods; cannot be downgraded by a crafted token header.
//   - Expiry is required (jwt.WithExpirationRequired).
//   - Issued-at is validated to prevent tokens from the future (jwt.WithIssuedAt).
//   - 5-second clock leeway handles minor clock skew across services.
func Auth(cfg *config.Config) fiber.Handler {
	signingKey := []byte(cfg.JWTSecret)

	parserOptions := []jwt.ParserOption{
		// Pin to HS256 — prevents algorithm confusion attacks (e.g. "none", RSA→HMAC).
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		// Reject tokens without an expiry claim entirely.
		jwt.WithExpirationRequired(),
		// Reject tokens whose iat is in the future (prevents pre-issued abuse).
		jwt.WithIssuedAt(),
		// Tolerate up to 5 s of clock skew between services.
		jwt.WithLeeway(5 * time.Second),
	}

	return func(c *fiber.Ctx) error {
		raw := c.Get(constants.HeaderAuthorization)
		if raw == "" {
			return response.Error(c,
				fiber.StatusUnauthorized,
				constants.MsgMissingAuthHeader,
				constants.ErrCodeMissingToken,
			)
		}

		parts := strings.SplitN(raw, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return response.Error(c,
				fiber.StatusUnauthorized,
				constants.MsgTokenInvalid,
				constants.ErrCodeTokenInvalid,
			)
		}

		claims := &jwtClaims{}
		token, err := jwt.ParseWithClaims(
			parts[1],
			claims,
			func(t *jwt.Token) (interface{}, error) {
				return signingKey, nil
			},
			parserOptions...,
		)

		if err != nil {
			// Surface the most actionable error to the client without leaking internals.
			switch {
			case isExpiredError(err):
				return response.Error(c,
					fiber.StatusUnauthorized,
					constants.MsgTokenExpired,
					constants.ErrCodeTokenExpired,
				)
			default:
				return response.Error(c,
					fiber.StatusUnauthorized,
					constants.MsgTokenInvalid,
					constants.ErrCodeTokenInvalid,
				)
			}
		}

		if !token.Valid || claims.UserID == "" {
			return response.Error(c,
				fiber.StatusUnauthorized,
				constants.MsgTokenInvalid,
				constants.ErrCodeTokenInvalid,
			)
		}

		// Inject into Locals for middleware/handlers in this process.
		c.Locals(constants.LocalUserID, claims.UserID)
		c.Locals(constants.LocalUserRole, claims.Role)

		// Inject into request headers so downstream gRPC/HTTP services can trust identity
		// without re-validating the token (gateway is the trust boundary).
		c.Request().Header.Set(constants.HeaderUserID, claims.UserID)
		c.Request().Header.Set(constants.HeaderUserRole, claims.Role)

		return c.Next()
	}
}

// RequireRole returns a middleware that enforces role-based access control.
// Must be placed after Auth in the middleware chain.
// Usage: app.Post("/admin/...", middleware.Auth(cfg), middleware.RequireRole("admin"))
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[strings.ToLower(r)] = struct{}{}
	}

	return func(c *fiber.Ctx) error {
		role, _ := c.Locals(constants.LocalUserRole).(string)
		if _, ok := allowed[strings.ToLower(role)]; !ok {
			return response.Error(c,
				fiber.StatusForbidden,
				constants.MsgForbiddenRole,
				constants.ErrCodeForbiddenRole,
			)
		}
		return c.Next()
	}
}

func isExpiredError(err error) bool {
	return strings.Contains(err.Error(), "token is expired")
}
