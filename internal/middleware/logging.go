package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/metrics"
)

// Logging is a structured Zap middleware that replaces Fiber's built-in logger.
// Every request emits two log lines:
//   - request start (debug) — provides an audit trail before the handler runs
//   - request end  (info/error) — final status + latency
//
// Log format matches the spec:
//
//	{ "time":"...", "method":"...", "path":"...", "status":"...",
//	  "latency":"...", "ip":"...", "request_id":"...", "user_id":"..." }
func Logging(log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		requestID, _ := c.Locals(constants.LocalRequestID).(string)
		userID, _ := c.Locals(constants.LocalUserID).(string)

		// Attach a per-request child logger enriched with trace fields.
		reqLog := log.With(
			zap.String(constants.LocalRequestID, requestID),
			zap.String(constants.LocalUserID, userID),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
		)

		// Store the enriched logger so handlers can use it without re-building fields.
		c.Locals(constants.LocalLogger, reqLog)

		reqLog.Debug(constants.MsgRequestReceived)

		// Increment active connection gauge.
		metrics.ActiveConnections.Inc()
		defer metrics.ActiveConnections.Dec()

		// Run the handler chain.
		err := c.Next()

		latency := time.Since(start)
		status := c.Response().StatusCode()

		// Prometheus: record request count + latency.
		statusStr := strconv.Itoa(status)
		metrics.RequestsTotal.WithLabelValues(c.Method(), c.Path(), statusStr).Inc()
		metrics.RequestDurationSeconds.WithLabelValues(c.Method(), c.Path()).Observe(latency.Seconds())

		fields := []zap.Field{
			zap.Int("status", status),
			zap.Duration("latency", latency),
		}

		if status >= 400 {
			metrics.ErrorsTotal.WithLabelValues(c.Method(), c.Path(), statusStr).Inc()
			if err != nil {
				fields = append(fields, zap.Error(err))
			}
			reqLog.Error(constants.MsgRequestCompleted, fields...)
		} else {
			reqLog.Info(constants.MsgRequestCompleted, fields...)
		}

		return err
	}
}
