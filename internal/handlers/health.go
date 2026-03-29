package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/grpc"
	"github.com/ShubhamMor21/go-gateway/internal/logger"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

// HealthStatus is the JSON body of the /health response.
type HealthStatus struct {
	Status     string            `json:"status"`
	Message    string            `json:"message"`
	Components map[string]string `json:"components"`
}

// Health performs readiness checks against all critical dependencies and
// returns a 200 (healthy) or 503 (degraded) response.
//
// Security: error details are intentionally hidden from the response body.
// The /health endpoint is unauthenticated so any caller can reach it — detailed
// error messages (Redis connection strings, internal hostnames) would leak
// infrastructure topology. Errors are logged server-side at warn level.
func Health(cacheClient *cache.Client, grpcClient *grpc.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		log := logger.FromContext(c.Locals(constants.LocalLogger))

		components := make(map[string]string)
		healthy := true

		// Check Redis — only log the error; never expose it in the response.
		if err := cacheClient.Ping(context.Background()); err != nil {
			components["redis"] = "unhealthy"
			healthy = false
			log.Warn("health check: redis unhealthy", zap.Error(err))
		} else {
			components["redis"] = "healthy"
		}

		// Check gRPC circuit breaker state.
		if grpcClient != nil {
			state := grpcClient.Breaker()
			components["grpc"] = state
			if state != "closed" {
				healthy = false
				log.Warn("health check: grpc circuit breaker not closed",
					zap.String("state", state))
			}
		}

		if healthy {
			return response.Success(c, fiber.StatusOK, HealthStatus{
				Status:     "healthy",
				Message:    constants.MsgHealthOK,
				Components: components,
			})
		}

		return response.Error(c,
			fiber.StatusServiceUnavailable,
			constants.MsgHealthDegraded,
			constants.ErrCodeServiceUnavailable,
		)
	}
}
