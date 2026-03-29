package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ShubhamMor21/go-gateway/internal/cache"
	"github.com/ShubhamMor21/go-gateway/internal/config"
	"github.com/ShubhamMor21/go-gateway/internal/constants"
	grpcclient "github.com/ShubhamMor21/go-gateway/internal/grpc"
	applogger "github.com/ShubhamMor21/go-gateway/internal/logger"
	"github.com/ShubhamMor21/go-gateway/internal/metrics"
	"github.com/ShubhamMor21/go-gateway/internal/middleware"
	"github.com/ShubhamMor21/go-gateway/internal/queue"
	"github.com/ShubhamMor21/go-gateway/internal/response"
	"github.com/ShubhamMor21/go-gateway/internal/router"
)

func main() {
	// ── Config ────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("fatal: config load failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	// ── Logger ────────────────────────────────────────────────────────
	log, err := applogger.New(cfg.LogLevel)
	if err != nil {
		_, _ = os.Stderr.WriteString("fatal: logger init failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer log.Sync() //nolint:errcheck

	// ── Redis ─────────────────────────────────────────────────────────
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("redis: invalid URL", zap.Error(err))
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatal("redis: connection failed", zap.Error(err))
	}
	log.Info("redis: connected")
	cacheClient := cache.New(rdb)

	// ── gRPC client ───────────────────────────────────────────────────
	var grpcClient *grpcclient.Client
	if cfg.GRPCServiceURL != "" {
		grpcClient, err = grpcclient.New(cfg, log)
		if err != nil {
			log.Warn("grpc: client init failed — health check will report degraded", zap.Error(err))
		} else {
			defer grpcClient.Close()
			log.Info("grpc: client ready",
				zap.String("url", cfg.GRPCServiceURL),
				zap.Bool("tls", cfg.GRPCTLSEnabled),
			)
		}
	}

	// ── Kafka producer ────────────────────────────────────────────────
	// Kafka is initialised here and injected into any handler that needs to
	// publish domain events. Wire it into router.Register when a handler needs it.
	if len(cfg.KafkaBrokers) > 0 {
		producer, err := queue.NewProducer(cfg, log)
		if err != nil {
			log.Warn("kafka: producer init failed", zap.Error(err))
		} else {
			defer producer.Close()
			log.Info("kafka: producer ready",
				zap.Bool("tls", cfg.KafkaTLSEnabled),
				zap.Bool("sasl", cfg.KafkaSASLEnabled),
			)
		}
	}

	// ── Fiber ─────────────────────────────────────────────────────────
	enableStackTrace := strings.ToLower(cfg.LogLevel) == "debug"

	app := fiber.New(fiber.Config{
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		IdleTimeout:           cfg.IdleTimeout,
		Concurrency:           cfg.MaxConcurrency,
		BodyLimit:             cfg.MaxRequestSize,
		DisableStartupMessage: false,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return response.Error(c, code, constants.MsgInternalError, constants.ErrCodeInternalError)
		},
	})

	// ── Global middleware (order matters) ─────────────────────────────
	app.Use(recover.New(recover.Config{EnableStackTrace: enableStackTrace}))
	app.Use(middleware.RequestID())
	app.Use(middleware.Security(cfg))
	app.Use(middleware.CORS(cfg))
	app.Use(middleware.LoadShedding(cfg))
	app.Use(middleware.Timeout(cfg))
	app.Use(middleware.Logging(log))

	// ── Routes ────────────────────────────────────────────────────────
	router.Register(app, cfg, cacheClient, grpcClient)

	// ── Prometheus metrics server (separate port) ─────────────────────
	metricsServer := &http.Server{
		Addr:    ":" + cfg.MetricsPort,
		Handler: metricsHandler(cfg),
	}
	go func() {
		log.Info("metrics server listening",
			zap.String("port", cfg.MetricsPort),
			zap.Bool("auth_enabled", cfg.MetricsAuthToken != ""),
		)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info("shutdown signal received")

		if err := app.ShutdownWithTimeout(cfg.GracefulShutdownTimeout); err != nil {
			log.Error("fiber shutdown error", zap.Error(err))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
		defer cancel()
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Error("metrics server shutdown error", zap.Error(err))
		}

		log.Info("gateway stopped cleanly")
	}()

	// ── Start ─────────────────────────────────────────────────────────
	log.Info("gateway starting",
		zap.String("port", cfg.ServerPort),
		zap.String("log_level", cfg.LogLevel),
	)
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		log.Fatal("gateway listen error", zap.Error(err))
	}
}

func metricsHandler(cfg *config.Config) http.Handler {
	prom := metrics.ServeHTTP()
	if cfg.MetricsAuthToken == "" {
		return prom
	}
	expected := "Bearer " + cfg.MetricsAuthToken
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		prom.ServeHTTP(w, r)
	})
}
