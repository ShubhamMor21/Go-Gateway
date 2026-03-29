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

// testSecret is used only in tests; it is never hardcoded in production paths.
const testSecret = "test-secret-for-unit-tests-only"

func newTestConfig() *config.Config {
	return &config.Config{
		JWTSecret: testSecret,
	}
}

func buildToken(t *testing.T, userID, role string, expiry time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
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

func newAuthApp(cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.Auth(cfg))
	app.Get("/protected", func(c *fiber.Ctx) error {
		uid, _ := c.Locals(constants.LocalUserID).(string)
		return c.JSON(fiber.Map{"user_id": uid})
	})
	return app
}

func TestAuth_ValidToken(t *testing.T) {
	cfg := newTestConfig()
	app := newAuthApp(cfg)
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
	cfg := newTestConfig()
	app := newAuthApp(cfg)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	cfg := newTestConfig()
	app := newAuthApp(cfg)
	token := buildToken(t, "user-123", "user", -time.Minute) // already expired

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(constants.HeaderAuthorization, "Bearer "+token)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_InvalidSignature(t *testing.T) {
	cfg := newTestConfig()
	app := newAuthApp(cfg)

	// Signed with a different secret.
	claims := jwt.MapClaims{"sub": "user-999", "exp": time.Now().Add(time.Hour).Unix()}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	badToken, _ := tok.SignedString([]byte("wrong-secret"))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(constants.HeaderAuthorization, "Bearer "+badToken)
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_MalformedBearerFormat(t *testing.T) {
	cfg := newTestConfig()
	app := newAuthApp(cfg)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(constants.HeaderAuthorization, "Token notabearer")
	resp, _ := app.Test(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}
