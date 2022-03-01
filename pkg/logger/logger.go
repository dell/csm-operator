package logger

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

// LogLevel represents the level for the log.
type LogLevel string

const (
	// ProductionLogLevel is the level for the production log.
	ProductionLogLevel LogLevel = "PRODUCTION"
	// DevelopmentLogLevel is the level for development log.
	DevelopmentLogLevel LogLevel = "DEVELOPMENT"
	// EnvLoggerLevel is the environment variable name for log level.
	EnvLoggerLevel = "LOGGER_LEVEL"
	// LogCtxIDKey holds the TraceId for log.
	LogCtxIDKey = "TraceId"
)

var defaultLogLevel LogLevel

// loggerKey holds the context key used for loggers.
type loggerKey struct{}

// SetLoggerLevel helps set defaultLogLevel, using which newLogger func helps
// create either development logger or production logger
func SetLoggerLevel(logLevel LogLevel) {
	defaultLogLevel = logLevel
}

// getLogger returns the logger associated with the given context.
// If there is no logger associated with context, getLogger func will return
// a new logger.
func getLogger(ctx context.Context) *zap.Logger {
	if logger, _ := ctx.Value(loggerKey{}).(*zap.Logger); logger != nil {
		return logger
	}
	return newLogger()
}

// GetLogger returns SugaredLogger associated with given context.
func GetLogger(ctx context.Context) *zap.SugaredLogger {
	return getLogger(ctx).Sugar()
}

// NewContextWithLogger returns a new child context with context ID set
func NewContextWithLogger(ctx context.Context, key string) context.Context {
	newCtx := withFields(ctx, zap.String(LogCtxIDKey, key))
	return newCtx
}

// GetNewContextWithLogger creates a new context with context UUID and logger
// set func returns both context and logger to the caller.
func GetNewContextWithLogger(key string) (context.Context, *zap.SugaredLogger) {
	newCtx := NewContextWithLogger(context.Background(), key)
	return newCtx, GetLogger(newCtx)
}

// withFields returns a new context derived from ctx
// that has a logger that always logs the given fields.
func withFields(ctx context.Context, fields ...zapcore.Field) context.Context {
	return context.WithValue(ctx, loggerKey{}, getLogger(ctx).With(fields...))
}

// newLogger creates and return a new logger depending logLevel set.
func newLogger() *zap.Logger {
	pe := zap.NewProductionEncoderConfig()
	pe.EncodeTime = zapcore.ISO8601TimeEncoder
	pe.EncodeLevel = zapcore.CapitalLevelEncoder

	pe.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(pe)

	level := zap.InfoLevel
	level = zap.DebugLevel

	core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), level)

	l := zap.New(core, zap.AddCaller())
	return l
}
