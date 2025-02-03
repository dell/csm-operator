package logger

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestGetLogger(t *testing.T) {
	// Test case: No logger in context
	ctx := context.Background()
	l := GetLogger(ctx)
	if l == nil {
		t.Error("Expected non-nil logger, but got nil")
	}

	// Test case: Logger in context
	ctx = context.WithValue(ctx, loggerKey{}, zap.NewNop())
	l = GetLogger(ctx)
	if l == nil {
		t.Error("Expected non-nil logger, but got nil")
	}
}

/*
func TestGetNewContextWithLogger(t *testing.T) {
	// Test case: No logger in context
	ctx := context.Background()
	newCtx, l := GetNewContextWithLogger(ctx, "test")
	if l == nil {
		t.Error("Expected non-nil logger, but got nil")
	}
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}

	// Test case: Logger in context
	ctx = context.WithValue(ctx, loggerKey{}, zap.NewNop())
	newCtx, l = GetNewContextWithLogger(ctx, "test")
	if l == nil {
		t.Error("Expected non-nil logger, but got nil")
	}
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}
}

func TestNewContextWithLogger(t *testing.T) {
	// Test case: No logger in context
	ctx := context.Background()
	newCtx := NewContextWithLogger(ctx, "test")
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}

	// Test case: Logger in context
	ctx = context.WithValue(ctx, loggerKey{}, zap.NewNop())
	newCtx = NewContextWithLogger(ctx, "test")
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}
}

func TestWithFields(t *testing.T) {
	// Test case: No logger in context
	ctx := context.Background()
	newCtx := WithFields(ctx, zap.String("key", "value"))
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}

	// Test case: Logger in context
	ctx = context.WithValue(ctx, loggerKey{}, zap.NewNop())
	newCtx = WithFields(ctx, zap.String("key", "value"))
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}
}

func TestNewLogger(t *testing.T) {
	// Test case: Production log level
	os.Setenv("LOGGER_LEVEL", "PRODUCTION")
	l, err := newLogger()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if l.Core().Enabled(zapcore.DebugLevel) {
		t.Error("Expected debug level to be disabled, but it is enabled")
	}

	// Test case: Development log level
	os.Setenv("LOGGER_LEVEL", "DEVELOPMENT")
	l, err = newLogger()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if !l.Core().Enabled(zapcore.DebugLevel) {
		t.Error("Expected debug level to be enabled, but it is disabled")
	}

	// Test case: Invalid log level
	os.Setenv("LOGGER_LEVEL", "INVALID")
	l, err = newLogger()
	if err == nil {
		t.Error("Expected error, but got nil")
	}
}
*/
