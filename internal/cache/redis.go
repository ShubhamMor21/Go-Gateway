package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Client wraps a Redis connection with caching helpers.
// singleflight collapses concurrent cache-miss fetches for the same key,
// preventing cache stampede under high concurrency.
type Client struct {
	rdb   *redis.Client
	group singleflight.Group
}

// New creates a Client from a pre-established *redis.Client.
func New(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// Ping verifies the Redis connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Get fetches a cached value. Returns (nil, nil) on a cache miss.
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

// Set serialises value to JSON and stores it with the given TTL.
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal %q: %w", key, err)
	}
	if err := c.rdb.Set(ctx, key, raw, ttl).Err(); err != nil {
		return fmt.Errorf("cache set %q: %w", key, err)
	}
	return nil
}

// Delete removes a single key.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// GetOrSet returns the cached value for key; if absent, it calls fetch exactly once
// (even if N goroutines are waiting), stores the result, and returns it.
// This eliminates cache stampedes under high concurrency.
func (c *Client) GetOrSet(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fetch func() (interface{}, error),
) (interface{}, error) {
	// Fast path: cache hit.
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var result interface{}
		if err := json.Unmarshal(raw, &result); err == nil {
			return result, nil
		}
	}

	// Slow path: deduplicated fetch.
	val, err, _ := c.group.Do(key, func() (interface{}, error) {
		// Double-check after acquiring the singleflight slot.
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

		// Best-effort write; do not fail the request on a cache write error.
		_ = c.Set(ctx, key, result, ttl)
		return result, nil
	})

	return val, err
}

// CacheKey builds a namespaced cache key: "<userID>:<endpoint>".
// Both segments come from callers — no hardcoded strings here.
func CacheKey(userID, endpoint string) string {
	return userID + ":" + endpoint
}

// RDB exposes the underlying Redis client for use by the rate-limiter Lua scripts.
func (c *Client) RDB() *redis.Client {
	return c.rdb
}
