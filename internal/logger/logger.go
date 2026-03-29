package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a production-grade Zap logger.
// logLevel is read from config (ENV); valid values: debug, info, warn, error.
func New(logLevel string) (*zap.Logger, error) {
	level, err := parseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("logger: invalid log level %q: %w", logLevel, err)
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}

	log, err := cfg.Build(zap.AddCallerSkip(0))
	if err != nil {
		return nil, fmt.Errorf("logger: build failed: %w", err)
	}

	return log, nil
}

// FromContext extracts a *zap.Logger stored in Fiber locals (LocalLogger key).
// Falls back to the global logger if not found.
func FromContext(localValue interface{}) *zap.Logger {
	if log, ok := localValue.(*zap.Logger); ok {
		return log
	}
	// Fall back to a no-op logger so callers never need to nil-check.
	return zap.NewNop()
}

func parseLevel(s string) (zapcore.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info", "":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown level %q", s)
	}
}
