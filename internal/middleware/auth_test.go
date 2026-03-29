package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/middleware"
)

const testSecret = "test-secret-for-unit-tests-only-32b"

func newTestConfig() *config.Config {
	return &config.Config{
		JWTSecret:    testSecret,
		JWTAlgorithm: "HS256",
	}
}

func buildToken(t *testing.T, userID, role string, expiry time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"iat":  time.Now().Unix(),
	}
	if expiry != 0 {
		claims["exp"] = time.Now().Add(expiry).Unix()
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("buildToken: %v", err)
	}
	return signed
}

// newAuthApp passes nil for cacheClient — skips revocation check in unit tests.
// Integration tests that need revocation should pass a real *cache.Client.
func newAuthApp(cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.Auth(cfg, nil))
	app.Get("/protected", func(c *fiber.Ctx) error {
		uid, _ := c.Locals(constants.LocalUserID).(string)
		return c.JSON(fiber.Map{"user_id": uid})
	})
	return app
}

func TestAuth_ValidToken(t *testing.T) {
	app := newAuthApp(newTestConfig())
	token := buildToken(t, "user-123", "admin", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(constants.HeaderAuthorization, "Bearer "+token)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	app := newAuthApp(newTestConfig())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	app := newAuthApp(newTestConfig())
	token := buildToken(t, "user-123", "user", -time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(constants.HeaderAuthorization, "Bearer "+token)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_InvalidSignature(t *testing.T) {
	app := newAuthApp(newTestConfig())

	claims := jwt.MapClaims{
		"sub": "user-999",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	badToken, _ := tok.SignedString([]byte("wrong-secret-totally-different-key"))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(constants.HeaderAuthorization, "Bearer "+badToken)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_MalformedBearerFormat(t *testing.T) {
	app := newAuthApp(newTestConfig())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(constants.HeaderAuthorization, "Token notabearer")
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestAuth_RequireRole verifies that the RBAC middleware correctly gates
// endpoints by role.
func TestAuth_RequireRole(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.Auth(newTestConfig(), nil))
	app.Get("/admin", middleware.RequireRole("admin"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Admin token — should pass
	adminToken := buildToken(t, "user-1", "admin", time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set(constants.HeaderAuthorization, "Bearer "+adminToken)
	resp, _ := app.Test(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin role: expected 200, got %d", resp.StatusCode)
	}

	// Regular user token — should be rejected
	userToken := buildToken(t, "user-2", "user", time.Hour)
	req2 := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req2.Header.Set(constants.HeaderAuthorization, "Bearer "+userToken)
	resp2, _ := app.Test(req2)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("user role: expected 403, got %d", resp2.StatusCode)
	}
}
