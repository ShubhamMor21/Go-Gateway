package circuitbreaker

import (
	"errors"
	"time"

	"github.com/sony/gobreaker"

	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/metrics"
)

// ErrCircuitOpen is returned when the breaker is in the open state.
var ErrCircuitOpen = errors.New(constants.MsgCircuitOpen)

// Settings mirrors gobreaker.Settings but uses our Config types.
type Settings struct {
	Name          string
	MaxRequests   uint32
	Interval      time.Duration
	Timeout       time.Duration
	FailureRatio  float64
}

// Breaker wraps gobreaker.CircuitBreaker with Prometheus instrumentation.
type Breaker struct {
	cb      *gobreaker.CircuitBreaker
	service string
}

// New creates a Breaker for a named downstream service.
// On every state change the Prometheus gauge is updated so Grafana alerts fire.
func New(s Settings) *Breaker {
	b := &Breaker{service: s.Name}

	b.cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        s.Name,
		MaxRequests: s.MaxRequests,
		Interval:    s.Interval,
		Timeout:     s.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= s.FailureRatio
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			metrics.CircuitBreakerState.WithLabelValues(name).Set(stateToFloat(to))
		},
	})

	// Initialise the metric as closed (0).
	metrics.CircuitBreakerState.WithLabelValues(s.Name).Set(0)

	return b
}

// Execute wraps fn with the circuit breaker. Returns ErrCircuitOpen when tripped.
func (b *Breaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	result, err := b.cb.Execute(fn)
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return nil, ErrCircuitOpen
	}
	return result, err
}

// State returns a human-readable string of the current breaker state.
func (b *Breaker) State() string {
	return b.cb.State().String()
}

func stateToFloat(s gobreaker.State) float64 {
	switch s {
	case gobreaker.StateClosed:
		return 0
	case gobreaker.StateOpen:
		return 1
	case gobreaker.StateHalfOpen:
		return 2
	}
	return 0
}
