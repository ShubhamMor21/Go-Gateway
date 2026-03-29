package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"github.com/ShubhamMor21/go-gateway/internal/constants"
)

// Client wraps a Redis connection with caching, token revocation, and IP blocklist helpers.
type Client struct {
	rdb   *redis.Client
	group singleflight.Group
}

func New(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// ──────────────────────────────────────────────
// Generic cache
// ──────────────────────────────────────────────

func (c *Client) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache get %q: %w", key, err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, fmt.Errorf("cache unmarshal %q: %w", key, err)
	}
	return true, nil
}

func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal %q: %w", key, err)
	}
	return c.rdb.Set(ctx, key, raw, ttl).Err()
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// GetOrSet returns a cached value; on miss it calls fetch exactly once
// (singleflight), stores the result, and returns it — preventing stampedes.
func (c *Client) GetOrSet(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fetch func() (interface{}, error),
) (interface{}, error) {
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var result interface{}
		if err := json.Unmarshal(raw, &result); err == nil {
			return result, nil
		}
	}

	val, err, _ := c.group.Do(key, func() (interface{}, error) {
		raw, err := c.rdb.Get(ctx, key).Bytes()
		if err == nil {
			var result interface{}
			if jsonErr := json.Unmarshal(raw, &result); jsonErr == nil {
				return result, nil
			}
		}
		result, err := fetch()
		if err != nil {
			return nil, err
		}
		_ = c.Set(ctx, key, result, ttl)
		return result, nil
	})
	return val, err
}

func CacheKey(userID, endpoint string) string {
	return userID + ":" + endpoint
}

func (c *Client) RDB() *redis.Client {
	return c.rdb
}

// ──────────────────────────────────────────────
// [CRITICAL] Token revocation
// ──────────────────────────────────────────────
// Tokens are revoked by storing a SHA-256 hash of the raw token string.
// This works regardless of whether the issuer includes a jti claim.
// Key: constants.RedisKeyRevokedPrefix + tokenHash
// TTL: remaining lifetime of the token (so keys expire automatically).

// RevokeToken marks a token hash as revoked for the remaining token lifetime.
func (c *Client) RevokeToken(ctx context.Context, tokenHash string, ttl time.Duration) error {
	if ttl <= 0 {
		// Token already expired — nothing to revoke.
		return nil
	}
	key := constants.RedisKeyRevokedPrefix + tokenHash
	return c.rdb.Set(ctx, key, "1", ttl).Err()
}

// IsRevoked returns true if the token hash exists in the revocation store.
// On Redis error it returns (false, err) — callers decide fail-open vs fail-closed.
func (c *Client) IsRevoked(ctx context.Context, tokenHash string) (bool, error) {
	key := constants.RedisKeyRevokedPrefix + tokenHash
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ──────────────────────────────────────────────
// [MEDIUM] IP blocklist
// ──────────────────────────────────────────────
// IPs are stored in a Redis SET at constants.RedisKeyBlockedIPs.
// Populate via Redis CLI: SADD blocked_ips 1.2.3.4
// Or via the BlockIP method in an admin handler.

// IsIPBlocked returns true if the IP is in the blocklist.
// On Redis error it returns (false, err) — caller should fail open to avoid
// taking the gateway down when the blocklist store is unreachable.
func (c *Client) IsIPBlocked(ctx context.Context, ip string) (bool, error) {
	blocked, err := c.rdb.SIsMember(ctx, constants.RedisKeyBlockedIPs, ip).Result()
	if err != nil {
		return false, err
	}
	return blocked, nil
}

// BlockIP adds an IP to the blocklist. The entry persists until explicitly removed.
func (c *Client) BlockIP(ctx context.Context, ip string) error {
	return c.rdb.SAdd(ctx, constants.RedisKeyBlockedIPs, ip).Err()
}

// UnblockIP removes an IP from the blocklist.
func (c *Client) UnblockIP(ctx context.Context, ip string) error {
	return c.rdb.SRem(ctx, constants.RedisKeyBlockedIPs, ip).Err()
}
