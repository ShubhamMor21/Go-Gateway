package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// Security attaches hardened HTTP response headers to every outbound response.
// All header names come from constants/headers.go — no inline strings.
func Security(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Content-Security-Policy: restrict resource origins aggressively.
		c.Set(constants.HeaderCSP,
			"default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none';",
		)

		// HSTS: force TLS for 1 year, include subdomains.
		c.Set(constants.HeaderHSTS, "max-age=31536000; includeSubDomains; preload")

		// Prevent clickjacking.
		c.Set(constants.HeaderXFrameOptions, "DENY")

		// Prevent MIME-type sniffing.
		c.Set(constants.HeaderXContentType, "nosniff")

		// Legacy XSS filter — belt-and-suspenders for older browsers.
		c.Set(constants.HeaderXXSSProtection, "1; mode=block")

		// Do not leak Referer header to third-party origins.
		c.Set(constants.HeaderReferrerPolicy, "strict-origin-when-cross-origin")

		// Disable browser features not needed by an API gateway.
		c.Set(constants.HeaderPermissionsPolicy, "geolocation=(), microphone=(), camera=()")

		// Remove the Server header to avoid fingerprinting.
		c.Set("Server", "")

		return c.Next()
	}
}

// CORS applies ENV-driven cross-origin resource sharing headers.
//
// Security fix: we no longer reflect the Origin header back when the configured
// value is "*". Reflecting the origin with credentials is a CORS misconfiguration
// that allows cross-site requests from any domain.
// Rules:
//   - CORS_ALLOWED_ORIGINS=*      → respond with literal "Access-Control-Allow-Origin: *"
//                                   (browsers block this with credentialed requests anyway)
//   - CORS_ALLOWED_ORIGINS=<url>  → only set the header when the request origin matches exactly
//   - Multiple origins             → comma-separated list checked against request origin
func CORS(cfg *config.Config) fiber.Handler {
	allowedOrigins := parseCORSOrigins(cfg.CORSAllowedOrigins)
	wildcard := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

	return func(c *fiber.Ctx) error {
		if wildcard {
			// Literal wildcard — credentials (cookies) cannot be sent cross-origin with this.
			c.Set("Access-Control-Allow-Origin", "*")
		} else {
			requestOrigin := c.Get("Origin")
			if requestOrigin != "" && isOriginAllowed(requestOrigin, allowedOrigins) {
				c.Set("Access-Control-Allow-Origin", requestOrigin)
				c.Set("Vary", "Origin")
			}
		}

		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers",
			fmt.Sprintf("%s,%s,%s,%s",
				constants.HeaderAuthorization,
				constants.HeaderContentType,
				constants.HeaderRequestID,
				constants.HeaderIdempotencyKey,
			),
		)
		c.Set("Access-Control-Expose-Headers", constants.HeaderRequestID)
		c.Set("Access-Control-Max-Age", "86400")

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// RequireJSON rejects requests without Content-Type: application/json.
// Apply only to POST/PUT/PATCH routes that expect a JSON body.
func RequireJSON() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodPost ||
			c.Method() == fiber.MethodPut ||
			c.Method() == fiber.MethodPatch {
			ct := c.Get(constants.HeaderContentType)
			if !strings.HasPrefix(strings.ToLower(ct), "application/json") {
				return response.Error(c,
					fiber.StatusUnsupportedMediaType,
					constants.MsgInvalidContentType,
					constants.ErrCodeInvalidContentType,
				)
			}
		}
		return c.Next()
	}
}

func parseCORSOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func isOriginAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}
