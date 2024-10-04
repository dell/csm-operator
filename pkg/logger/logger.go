//  Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

// loggerKey holds the context key used for loggers.
type loggerKey struct{}

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
	//level = zap.DebugLevel

	core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), level)

	l := zap.New(core, zap.AddCaller())
	return l
}
