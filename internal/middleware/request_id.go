package middleware

import (
	"regexp"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// requestIDPattern only allows alphanumeric characters and hyphens (UUID format).
// This prevents log injection, cache-key injection, and metrics cardinality explosion.
var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9\-]{1,128}$`)

// RequestID generates a UUID v4 request ID for every inbound request.
// It reads a pre-existing X-Request-ID header first (set by load balancers or
// API clients) so that end-to-end traces remain continuous.
//
// Security: an incoming X-Request-ID is validated against a strict pattern before
// being trusted. Invalid IDs are silently replaced with a fresh UUID — we do not
// reject the request, which would break backwards compatibility with clients that
// send IDs in other formats.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := c.Get(constants.HeaderRequestID)

		if reqID != "" && !requestIDPattern.MatchString(reqID) {
			// Do not reject — replace with a gateway-generated ID and continue.
			// Returning an error here would break any client that sends a non-UUID
			// correlation ID in a non-standard format.
			reqID = uuid.New().String()
		}

		if reqID == "" {
			reqID = uuid.New().String()
		}

		c.Locals(constants.LocalRequestID, reqID)
		c.Set(constants.HeaderRequestID, reqID)

		return c.Next()
	}
}

// ValidateRequestID is a stricter variant that REJECTS requests with an invalid
// X-Request-ID header. Use on internal service-to-service routes where all callers
// are trusted and known to send valid IDs.
func ValidateRequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := c.Get(constants.HeaderRequestID)
		if reqID != "" && !requestIDPattern.MatchString(reqID) {
			return response.Error(c,
				fiber.StatusBadRequest,
				constants.MsgInvalidRequestID,
				constants.ErrCodeInvalidRequestID,
			)
		}
		return c.Next()
	}
}
