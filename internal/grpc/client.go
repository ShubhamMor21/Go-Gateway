// Package grpc provides a singleton gRPC client with connection pooling,
// circuit breaking, exponential-backoff retry, and metadata propagation.
//
// ─── Production wiring ────────────────────────────────────────────────────────
// Replace StubUserService (below) with the protobuf-generated client:
//
//	import userpb "github.com/ShubhamMor21/go-gateway/pkg/proto/user"
//	var _ UserService = userpb.NewUserServiceClient(conn)
//
// Run: go generate ./pkg/proto/... to regenerate from user.proto.
// ──────────────────────────────────────────────────────────────────────────────
package grpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/ShubhamMor21/go-gateway/internal/circuitbreaker"
	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
)

// ──────────────────────────────────────────────
// Domain types (replace with proto-generated types in production)
// ──────────────────────────────────────────────

// UserRequest is the gateway-internal request type for the User service.
type UserRequest struct {
	UserID    string
	RequestID string
}

// UserResponse is the gateway-internal response type for the User service.
type UserResponse struct {
	UserID string
	Name   string
	Email  string
	Role   string
}

// UserService is the interface the gateway calls.
// The protobuf-generated client satisfies this interface automatically once wired.
type UserService interface {
	GetUser(ctx context.Context, req *UserRequest) (*UserResponse, error)
}

// ──────────────────────────────────────────────
// Client
// ──────────────────────────────────────────────

// Client manages a single gRPC connection shared across all requests.
type Client struct {
	conn    *grpc.ClientConn
	user    UserService
	breaker *circuitbreaker.Breaker
	cfg     *config.Config
	log     *zap.Logger
}

// New dials the downstream gRPC service and returns a ready Client.
//
// TLS behaviour (controlled by GRPC_TLS_ENABLED env var):
//   - true  → uses the system CA bundle for server certificate validation.
//             Set GRPC_SERVER_NAME_OVERRIDE for self-signed certs in staging.
//   - false → insecure (plaintext). Only acceptable inside a private service mesh
//             with mTLS at the infrastructure layer (e.g. Istio, Linkerd).
func New(cfg *config.Config, log *zap.Logger) (*Client, error) {
	if cfg.GRPCServiceURL == "" {
		return nil, errors.New("GRPC_SERVICE_URL is not configured")
	}

	dialOpts, err := buildDialOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("grpc: build dial options: %w", err)
	}

	conn, err := grpc.NewClient(cfg.GRPCServiceURL, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %q: %w", cfg.GRPCServiceURL, err)
	}

	breaker := circuitbreaker.New(circuitbreaker.Settings{
		Name:         "user-service",
		MaxRequests:  cfg.CBMaxRequests,
		Interval:     cfg.CBInterval,
		Timeout:      cfg.CBTimeout,
		FailureRatio: cfg.CBFailureRatio,
	})

	// Swap StubUserService with userpb.NewUserServiceClient(conn) in production.
	userSvc := &StubUserService{}

	return &Client{
		conn:    conn,
		user:    userSvc,
		breaker: breaker,
		cfg:     cfg,
		log:     log,
	}, nil
}

// GetUser calls the downstream User service with circuit breaking + retry.
func (c *Client) GetUser(ctx context.Context, requestID, userID string) (*UserResponse, error) {
	// Propagate tracing metadata to the downstream gRPC service.
	ctx = metadata.AppendToOutgoingContext(ctx,
		constants.LocalRequestID, requestID,
		constants.LocalUserID, userID,
	)

	result, err := c.breaker.Execute(func() (interface{}, error) {
		return c.getUserWithRetry(ctx, &UserRequest{
			UserID:    userID,
			RequestID: requestID,
		})
	})

	if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	return result.(*UserResponse), nil
}

func (c *Client) getUserWithRetry(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.GRPCMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * c.cfg.GRPCRetryBaseDelay
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.user.GetUser(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		c.log.Warn(constants.MsgGRPCCallFailed,
			zap.String(constants.LocalRequestID, req.RequestID),
			zap.Int("attempt", attempt+1),
			zap.Error(err),
		)
	}
	return nil, fmt.Errorf("%s: %w", constants.MsgGRPCCallFailed, lastErr)
}

// Breaker returns the current circuit breaker state as a string ("closed", "open", "half-open").
func (c *Client) Breaker() string {
	return c.breaker.State()
}

// Close tears down the underlying connection. Call on graceful shutdown.
func (c *Client) Close() error {
	return c.conn.Close()
}

// ──────────────────────────────────────────────
// Dial option builder
// ──────────────────────────────────────────────

func buildDialOptions(cfg *config.Config) ([]grpc.DialOption, error) {
	if !cfg.GRPCTLSEnabled {
		// Plaintext — only safe inside a trusted service mesh with mTLS at infra layer.
		return []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}, nil
	}

	// TLS using the system CA pool (validates the server's certificate).
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13, // enforce TLS 1.3 minimum
	}

	if cfg.GRPCServerNameOverride != "" {
		// For staging environments with self-signed certs.
		// Do NOT set this in production.
		tlsCfg.ServerName = cfg.GRPCServerNameOverride
	}

	creds := credentials.NewTLS(tlsCfg)
	return []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}, nil
}

// ──────────────────────────────────────────────
// Stub — replace with proto-generated client
// ──────────────────────────────────────────────

// StubUserService is a placeholder implementation that returns mock data.
// In production, replace with the protobuf-generated NewUserServiceClient(conn).
type StubUserService struct{}

func (s *StubUserService) GetUser(_ context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		UserID: req.UserID,
		Name:   "Stub User",
		Email:  "stub@example.com",
		Role:   "user",
	}, nil
}
