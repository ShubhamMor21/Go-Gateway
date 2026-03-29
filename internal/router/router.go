package router

import (
	"github.com/gofiber/fiber/v2"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/config"
	grpcclient "github.com/ShubhamMor21/go-gateway/internal/grpc"
	"github.com/ShubhamMor21/go-gateway/internal/handlers"
	"github.com/ShubhamMor21/go-gateway/internal/middleware"
)

// Register attaches all routes. Global middleware (IP blocklist, rate limiter)
// is applied in main.go before this function runs — only auth-scoped middleware
// lives here.
func Register(
	app *fiber.App,
	cfg *config.Config,
	cacheClient *cache.Client,
	grpcClient *grpcclient.Client,
) {
	// ── Public ───────────────────────────────────────────────────────
	app.Get("/health", handlers.Health(cacheClient, grpcClient))

	// ── Authenticated API ─────────────────────────────────────────────
	// Rate limiting is applied globally (before auth) so unauthenticated
	// requests are throttled too — preventing brute-force on the auth endpoint.
	api := app.Group("/api/v1",
		middleware.Auth(cfg, cacheClient),
	)

	// Logout — must be authenticated; revokes the current token
	api.Post("/auth/logout", handlers.Logout(cacheClient))

	// User profile — [CRITICAL] ownership check inside the handler:
	// regular users can only fetch their own profile; admin role can fetch any.
	api.Get("/users/:id", handlers.GetUser(cacheClient, grpcClient, cfg))

	// ── [MEDIUM] Admin routes — role-gated ───────────────────────────
	// RequireRole("admin") rejects any token whose role claim != "admin".
	admin := api.Group("/admin", middleware.RequireRole("admin"))
	admin.Get("/status", func(c *fiber.Ctx) error {
		// Placeholder — wire real admin handlers here.
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "admin access confirmed",
		})
	})
}
