package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RequestsTotal counts every HTTP request by method, path, and status code.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed.",
		},
		[]string{"method", "path", "status"},
	)

	// RequestDurationSeconds tracks full request latency (handler + middleware).
	RequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// ErrorsTotal counts requests that resulted in a 4xx or 5xx response.
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "errors_total",
			Help:      "Total number of HTTP errors.",
		},
		[]string{"method", "path", "code"},
	)

	// ActiveConnections tracks in-flight requests at any instant.
	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "gateway",
		Name:      "active_connections",
		Help:      "Number of currently active connections.",
	})

	// CacheOperations counts cache hits and misses.
	CacheOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "cache_operations_total",
			Help:      "Cache hit/miss counts.",
		},
		[]string{"result"}, // "hit" | "miss"
	)

	// CircuitBreakerState tracks the circuit state per service (0=closed, 1=open, 2=half-open).
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "gateway",
			Name:      "circuit_breaker_state",
			Help:      "Circuit breaker state per downstream service (0=closed,1=open,2=half-open).",
		},
		[]string{"service"},
	)

	// QueuePublishTotal counts Kafka publish attempts.
	QueuePublishTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "queue_publish_total",
			Help:      "Kafka publish attempts by topic and result.",
		},
		[]string{"topic", "result"}, // result: "ok" | "error"
	)
)

// ServeHTTP returns a standard net/http handler for the /metrics endpoint.
// Expose this on the dedicated metrics port, not on the public API port.
func ServeHTTP() http.Handler {
	return promhttp.Handler()
}
