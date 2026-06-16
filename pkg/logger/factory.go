/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package logger provides logging utilities for the CSI driver
package logger

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewJSONLogger creates a new zap logger with JSON encoding for production
// This provides machine-readable structured logs with consistent formatting
func NewJSONLogger(serviceName string) (*zap.Logger, error) {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	encoderCfg.MessageKey = "msg"
	encoderCfg.CallerKey = "caller"
	encoderCfg.LevelKey = "level"

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller())

	if serviceName != "" {
		logger = logger.With(zap.String("service", serviceName))
	}

	atom.SetLevel(zap.InfoLevel)
	return logger, nil
}

// NewJSONLoggerOrNop creates a JSON logger or returns a no-op logger on error
// This is useful for init() functions where error handling is limited
func NewJSONLoggerOrNop(serviceName string) *zap.Logger {
	logger, err := NewJSONLogger(serviceName)
	if err != nil {
		return zap.NewNop()
	}
	return logger
}

// NewConsoleLogger creates a new zap logger with console encoding
// This provides human-readable logs with structured fields (for development/debugging)
// Deprecated: Use NewJSONLogger for production deployments
func NewConsoleLogger(serviceName string) (*zap.Logger, error) {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	encoderCfg.MessageKey = "msg"
	encoderCfg.CallerKey = "caller"
	encoderCfg.LevelKey = "level"

	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller())

	if serviceName != "" {
		logger = logger.With(zap.String("service", serviceName))
	}

	atom.SetLevel(zap.InfoLevel)
	return logger, nil
}

// NewConsoleLoggerOrNop creates a console logger or returns a no-op logger on error
// Deprecated: Use NewJSONLoggerOrNop for production deployments
func NewConsoleLoggerOrNop(serviceName string) *zap.Logger {
	logger, err := NewConsoleLogger(serviceName)
	if err != nil {
		return zap.NewNop()
	}
	return logger
}

// GenerateRequestID generates a unique request ID for tracing
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
