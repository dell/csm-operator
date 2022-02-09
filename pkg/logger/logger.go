package logger

import (
	"context"
	//"errors"
	//"fmt"
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
	if logLevel != ProductionLogLevel && logLevel != DevelopmentLogLevel {
		defaultLogLevel = ProductionLogLevel
	}
	//GetLoggerWithNoContext().Infof("Setting default log level to :%q", defaultLogLevel)
}

/*
func InitLogger() {
    writeSyncer := getLogWriter()
    encoder := getEncoder()
    core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)

    logger := zap.New(core)
    sugarLogger = logger.Sugar()
}
*/

func getEncoder() zapcore.Encoder {
	return zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
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

// NewContextWithLogger returns a new child context with context UUID set
// using key CtxId.
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

	//fileEncoder := zapcore.NewJSONEncoder(pe)
	fileEncoder := zapcore.NewConsoleEncoder(pe)

	pe.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(pe)

	level := zap.InfoLevel
	level = zap.DebugLevel

	stdoutSyncer := zapcore.Lock(os.Stdout)
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, stdoutSyncer, level),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
	)

	l := zap.New(core)

	return l
}

// GetLoggerWithNoContext returns a new logger to the caller.
// Returned logger is not associated with any context.
func GetLoggerWithNoContext() *zap.SugaredLogger {
	return newLogger().Sugar()
}
