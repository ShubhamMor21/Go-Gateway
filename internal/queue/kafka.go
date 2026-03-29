// Package queue provides a Kafka producer for publishing domain events.
// The gateway is a producer only — it never consumes queues.
package queue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	"github.com/ShubhamMor21/go-gateway/internal/metrics"
)

// Producer publishes messages to Kafka topics.
type Producer struct {
	writer *kafka.Writer
	cfg    *config.Config
	log    *zap.Logger
}

// Message is the canonical envelope published to every Kafka topic.
// request_id is mandatory for end-to-end traceability.
type Message struct {
	// IdempotencyKey must be set by the caller — it prevents duplicate processing
	// if the network publish is retried. Must be a valid UUID v4.
	IdempotencyKey string      `json:"idempotency_key"`
	RequestID      string      `json:"request_id"`
	Topic          string      `json:"topic"`
	EventType      string      `json:"event_type"`
	Payload        interface{} `json:"payload"`
	PublishedAt    time.Time   `json:"published_at"`
}

// NewProducer creates a Kafka writer with optional TLS and SASL authentication.
// Returns an error if no brokers are configured.
//
// Security:
//   - KAFKA_TLS_ENABLED=true  → encrypts all broker traffic (TLS 1.2 minimum).
//   - KAFKA_SASL_ENABLED=true → authenticates to the broker.
//     KAFKA_SASL_MECHANISM selects PLAIN, SCRAM-SHA-256, or SCRAM-SHA-512.
//   - In production both SHOULD be enabled. Running either without the other
//     risks either unencrypted credentials (SASL without TLS) or unauthenticated
//     writes (TLS without SASL).
func NewProducer(cfg *config.Config, log *zap.Logger) (*Producer, error) {
	if len(cfg.KafkaBrokers) == 0 {
		return nil, errors.New("KAFKA_BROKERS is required for queue producer")
	}

	transport, err := buildKafkaTransport(cfg)
	if err != nil {
		return nil, fmt.Errorf("kafka: build transport: %w", err)
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll, // strongest delivery guarantee
		MaxAttempts:  cfg.KafkaRetries,
		BatchTimeout: time.Duration(cfg.KafkaRetryBackoffMs) * time.Millisecond,
		Async:        false, // synchronous publish; callers can await errors
		Transport:    transport,
	}

	return &Producer{
		writer: writer,
		cfg:    cfg,
		log:    log,
	}, nil
}

// Publish serialises msg and writes it to the Kafka topic in msg.Topic.
// It retries with exponential backoff up to cfg.KafkaRetries attempts.
// IdempotencyKey must be a valid UUID (enforced by the caller / handler layer).
func (p *Producer) Publish(ctx context.Context, msg Message) error {
	if msg.IdempotencyKey == "" {
		return fmt.Errorf("%s: %s", constants.ErrCodeIdempotency, constants.MsgMissingIdempotencyKey)
	}
	if msg.RequestID == "" {
		return fmt.Errorf("request_id must be propagated to queue messages")
	}

	msg.PublishedAt = time.Now().UTC()

	raw, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("queue marshal: %w", err)
	}

	kmsg := kafka.Message{
		Topic: msg.Topic,
		Key:   []byte(msg.IdempotencyKey), // partition key = idempotency key for ordering
		Value: raw,
		Headers: []kafka.Header{
			{Key: constants.LocalRequestID, Value: []byte(msg.RequestID)},
		},
	}

	var lastErr error
	for attempt := 0; attempt <= p.cfg.KafkaRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) *
				time.Duration(p.cfg.KafkaRetryBackoffMs) * time.Millisecond
			select {
			case <-ctx.Done():
				metrics.QueuePublishTotal.WithLabelValues(msg.Topic, "error").Inc()
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		if err := p.writer.WriteMessages(ctx, kmsg); err == nil {
			metrics.QueuePublishTotal.WithLabelValues(msg.Topic, "ok").Inc()
			p.log.Info(constants.MsgPaymentQueued,
				zap.String("topic", msg.Topic),
				zap.String(constants.LocalRequestID, msg.RequestID),
				zap.String("idempotency_key", msg.IdempotencyKey),
			)
			return nil
		} else {
			lastErr = err
		}
	}

	metrics.QueuePublishTotal.WithLabelValues(msg.Topic, "error").Inc()
	p.log.Error(constants.MsgQueuePublishFail,
		zap.String("topic", msg.Topic),
		zap.String(constants.LocalRequestID, msg.RequestID),
		zap.Error(lastErr),
	)
	return fmt.Errorf("%s: %w", constants.MsgQueuePublishFail, lastErr)
}

// Close flushes pending messages and closes the writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// ──────────────────────────────────────────────
// Transport builder
// ──────────────────────────────────────────────

func buildKafkaTransport(cfg *config.Config) (*kafka.Transport, error) {
	transport := &kafka.Transport{}

	if cfg.KafkaTLSEnabled {
		transport.TLS = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	if cfg.KafkaSASLEnabled {
		mechanism, err := buildSASLMechanism(cfg)
		if err != nil {
			return nil, err
		}
		transport.SASL = mechanism
	}

	// If neither TLS nor SASL is configured, return nil so the writer uses the
	// default transport (no overhead).
	if !cfg.KafkaTLSEnabled && !cfg.KafkaSASLEnabled {
		return nil, nil
	}

	return transport, nil
}

func buildSASLMechanism(cfg *config.Config) (sasl.Mechanism, error) {
	switch cfg.KafkaSASLMechanism {
	case "PLAIN":
		return plain.Mechanism{
			Username: cfg.KafkaSASLUsername,
			Password: cfg.KafkaSASLPassword,
		}, nil
	case "SCRAM-SHA-256":
		return scram.Mechanism(scram.SHA256, cfg.KafkaSASLUsername, cfg.KafkaSASLPassword)
	case "SCRAM-SHA-512":
		return scram.Mechanism(scram.SHA512, cfg.KafkaSASLUsername, cfg.KafkaSASLPassword)
	default:
		return nil, fmt.Errorf("unsupported KAFKA_SASL_MECHANISM %q (use PLAIN, SCRAM-SHA-256, or SCRAM-SHA-512)", cfg.KafkaSASLMechanism)
	}
}
