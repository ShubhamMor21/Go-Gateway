package router

import (
	"github.com/gofiber/fiber/v2"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/config"
	grpcclient "github.com/ShubhamMor21/go-gateway/internal/grpc"
	"github.com/ShubhamMor21/go-gateway/internal/handlers"
	"github.com/ShubhamMor21/go-gateway/internal/middleware"
)

// Register attaches all routes and middleware to the Fiber app.
func Register(
	app *fiber.App,
	cfg *config.Config,
	cacheClient *cache.Client,
	grpcClient *grpcclient.Client,
) {
	// Public — no auth required (used by load balancers)
	app.Get("/health", handlers.Health(cacheClient, grpcClient))

	// Authenticated API group
	api := app.Group("/api/v1",
		middleware.Auth(cfg),
		middleware.RateLimiter(cfg, cacheClient.RDB()),
	)

	api.Get("/users/:id", handlers.GetUser(cacheClient, grpcClient, cfg))

	// Role-gated example (uncomment when admin routes are added):
	// admin := api.Group("/admin", middleware.RequireRole("admin"))
}
