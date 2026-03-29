package middleware

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/logger"
	"github.com/ShubhamMor21/go-gateway/internal/response"
)

type jwtClaims struct {
	UserID string `json:"sub"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Auth validates the JWT Bearer token and injects identity into Fiber Locals and
// downstream request headers.
//
// Algorithm support (JWT_ALGORITHM env var):
//   - HS256 (default) — symmetric HMAC. Requires JWT_SECRET ≥ 32 bytes.
//     Suitable when the gateway is the sole trust boundary.
//   - RS256            — asymmetric RSA. Requires JWT_PUBLIC_KEY_PATH or JWT_PUBLIC_KEY.
//     Recommended for distributed systems: downstream services verify with the
//     public key and NEVER need the private key.
//   - ES256            — asymmetric ECDSA (smaller key, faster verification than RSA).
//     Requires JWT_PUBLIC_KEY_PATH or JWT_PUBLIC_KEY.
//
// [CRITICAL] Token revocation:
//   Tokens are revoked by computing SHA-256(raw_token) and checking Redis.
//   The token hash and expiry are stored in Locals for the logout handler.
//   On Redis error the check fails-open (logs an error) to avoid taking down
//   all authenticated traffic when the cache layer degrades.
func Auth(cfg *config.Config, cacheClient *cache.Client) fiber.Handler {
	keyFunc, validMethods, initErr := buildKeyFunc(cfg)
	if initErr != nil {
		// Panic at startup — misconfigured auth is non-recoverable.
		panic(fmt.Sprintf("auth middleware: %v", initErr))
	}

	parserOptions := []jwt.ParserOption{
		jwt.WithValidMethods(validMethods),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5 * time.Second),
	}

	return func(c *fiber.Ctx) error {
		log := logger.FromContext(c.Locals(constants.LocalLogger))

		raw := c.Get(constants.HeaderAuthorization)
		if raw == "" {
			return response.Error(c,
				fiber.StatusUnauthorized,
				constants.MsgMissingAuthHeader,
				constants.ErrCodeMissingToken,
			)
		}

		parts := strings.SplitN(raw, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return response.Error(c,
				fiber.StatusUnauthorized,
				constants.MsgTokenInvalid,
				constants.ErrCodeTokenInvalid,
			)
		}

		tokenStr := parts[1]
		claims := &jwtClaims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, keyFunc, parserOptions...)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return response.Error(c,
					fiber.StatusUnauthorized,
					constants.MsgTokenExpired,
					constants.ErrCodeTokenExpired,
				)
			}
			return response.Error(c,
				fiber.StatusUnauthorized,
				constants.MsgTokenInvalid,
				constants.ErrCodeTokenInvalid,
			)
		}

		if !token.Valid || claims.UserID == "" {
			return response.Error(c,
				fiber.StatusUnauthorized,
				constants.MsgTokenInvalid,
				constants.ErrCodeTokenInvalid,
			)
		}

		// ── [CRITICAL] Revocation check ───────────────────────────────
		// Compute a deterministic hash of the raw token string.
		// This works regardless of whether the issuer includes a jti claim.
		hash := sha256.Sum256([]byte(tokenStr))
		tokenHash := hex.EncodeToString(hash[:])

		if cacheClient != nil {
			revoked, err := cacheClient.IsRevoked(c.UserContext(), tokenHash)
			if err != nil {
				// Fail-open on Redis error: log + continue.
				// Redis being down is already a P1 incident; adding auth failures
				// on top would make recovery harder.
				log.Error("revocation check failed — Redis unreachable, failing open",
					zap.String(constants.LocalRequestID, func() string {
						s, _ := c.Locals(constants.LocalRequestID).(string)
						return s
					}()),
					zap.Error(err),
				)
			} else if revoked {
				return response.Error(c,
					fiber.StatusUnauthorized,
					constants.MsgTokenRevoked,
					constants.ErrCodeTokenRevoked,
				)
			}
		}

		// ── Inject identity into Locals and downstream headers ────────
		c.Locals(constants.LocalUserID, claims.UserID)
		c.Locals(constants.LocalUserRole, claims.Role)
		c.Locals(constants.LocalTokenHash, tokenHash)

		// Store expiry time so the logout handler can compute the correct TTL.
		if claims.ExpiresAt != nil {
			c.Locals(constants.LocalTokenExp, claims.ExpiresAt.Time)
		}

		c.Request().Header.Set(constants.HeaderUserID, claims.UserID)
		c.Request().Header.Set(constants.HeaderUserRole, claims.Role)

		return c.Next()
	}
}

// RequireRole enforces role-based access control. Must follow Auth in the chain.
// Usage: api.Get("/admin/...", middleware.RequireRole("admin"))
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[strings.ToLower(r)] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals(constants.LocalUserRole).(string)
		if _, ok := allowed[strings.ToLower(role)]; !ok {
			return response.Error(c,
				fiber.StatusForbidden,
				constants.MsgForbiddenRole,
				constants.ErrCodeForbiddenRole,
			)
		}
		return c.Next()
	}
}

// ──────────────────────────────────────────────
// Key function builders per algorithm
// ──────────────────────────────────────────────

func buildKeyFunc(cfg *config.Config) (jwt.Keyfunc, []string, error) {
	switch cfg.JWTAlgorithm {
	case "RS256":
		key, err := parseRSAPublicKey(cfg.JWTPublicKeyPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("RS256 public key: %w", err)
		}
		return func(t *jwt.Token) (interface{}, error) { return key, nil },
			[]string{"RS256"}, nil

	case "ES256":
		key, err := parseECPublicKey(cfg.JWTPublicKeyPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("ES256 public key: %w", err)
		}
		return func(t *jwt.Token) (interface{}, error) { return key, nil },
			[]string{"ES256"}, nil

	default: // HS256
		signingKey := []byte(cfg.JWTSecret)
		return func(t *jwt.Token) (interface{}, error) { return signingKey, nil },
			[]string{"HS256"}, nil
	}
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key: %w", err)
	}
	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("PEM is not an RSA public key")
	}
	return rsaKey, nil
}

func parseECPublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key: %w", err)
	}
	ecKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("PEM is not an EC public key")
	}
	return ecKey, nil
}
