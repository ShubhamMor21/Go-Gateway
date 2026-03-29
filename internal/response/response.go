package response

import "github.com/gofiber/fiber/v2"

// Success wraps a successful payload in the standard envelope.
func Success(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

// Error sends a structured error response.
// message must come from constants/messages.go.
// code must come from constants/errors.go.
func Error(c *fiber.Ctx, status int, message, code string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"message": message,
		"code":    code,
	})
}

// ErrorWithDetails includes an optional details field (e.g. validation errors).
func ErrorWithDetails(c *fiber.Ctx, status int, message, code string, details interface{}) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"message": message,
		"code":    code,
		"details": details,
	})
}
