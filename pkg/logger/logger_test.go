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

func TestGetNewContextWithLogger(t *testing.T) {
	// Test case: No logger in context
	ctx := context.Background()
	newCtx, l := GetNewContextWithLogger("test")
	if l == nil {
		t.Error("Expected non-nil logger, but got nil")
	}
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}

	// Test case: Logger in context
	ctx = context.WithValue(ctx, loggerKey{}, zap.NewNop())
	newCtx, l = GetNewContextWithLogger("test")
	if l == nil {
		t.Error("Expected non-nil logger, but got nil")
	}
	if ctx == newCtx {
		t.Error("Expected new context, but got same context")
	}
}
