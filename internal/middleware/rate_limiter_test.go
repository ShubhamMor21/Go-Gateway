package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/middleware"
)

// newRedisForTest returns a Redis client pointed at the test instance.
// Set REDIS_URL in the test environment; skip the test if unavailable.
func newRedisForTest(t *testing.T) *redis.Client {
	t.Helper()

	url := "redis://localhost:6379"
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Skip("redis: invalid URL — skipping integration test")
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis: not reachable (%v) — skipping integration test", err)
	}
	return rdb
}

func newRateLimitApp(cfg *config.Config, rdb *redis.Client) *fiber.App {
	// ProxyHeader must be set so c.IP() reads from X-Forwarded-For.
	// Without this, all test requests share the same loopback IP and per-IP
	// limits cannot be tested in isolation.
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ProxyHeader:           "X-Forwarded-For",
	})
	app.Use(middleware.RateLimiter(cfg, rdb))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rdb := newRedisForTest(t)
	t.Cleanup(func() { rdb.Close() })

	cfg := &config.Config{
		RateLimitRequests:      5,
		RateLimitWindowSeconds: 60,
	}
	app := newRateLimitApp(cfg, rdb)

	// Flush any residual keys from previous runs.
	rdb.FlushDB(context.Background())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		resp, _ := app.Test(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rdb := newRedisForTest(t)
	t.Cleanup(func() { rdb.Close() })

	cfg := &config.Config{
		RateLimitRequests:      3,
		RateLimitWindowSeconds: 60,
	}
	app := newRateLimitApp(cfg, rdb)
	rdb.FlushDB(context.Background())

	ip := "10.0.0.2"
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", ip)
		resp, _ := app.Test(req)
		resp.Body.Close()
	}

	// The 4th request must be rejected.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", ip)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

func TestRateLimiter_PerUserIsolation(t *testing.T) {
	rdb := newRedisForTest(t)
	t.Cleanup(func() { rdb.Close() })

	cfg := &config.Config{
		RateLimitRequests:      2,
		RateLimitWindowSeconds: 60,
	}
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ProxyHeader:           "X-Forwarded-For",
	})
	app.Use(middleware.RateLimiter(cfg, rdb))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	rdb.FlushDB(context.Background())

	// Exhaust the per-IP limit for IP 10.0.0.3
	ip := "10.0.0.3"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", ip)
		resp, _ := app.Test(req)
		resp.Body.Close()
	}

	// A request from a different IP must still be allowed.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", 99))
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("different IP should not be rate-limited, got %d", resp.StatusCode)
	}
}
