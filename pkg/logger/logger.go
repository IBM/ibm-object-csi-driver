/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package logger provides utilities for structured logging with request ID tracking
package logger

import (
	"context"

	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"go.uber.org/zap"
)

// WithRequestID returns a logger with request ID field added
// If no request ID exists in context, returns the original logger
func WithRequestID(ctx context.Context, logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return nil
	}
	if reqID := requestid.FromContext(ctx); reqID != "" {
		return logger.With(zap.String("request_id", reqID))
	}
	return logger
}

// Info logs an info message with request ID from context
func Info(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Info(msg, fields...)
}

// Error logs an error message with request ID from context
func Error(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Error(msg, fields...)
}

// Warn logs a warning message with request ID from context
func Warn(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Warn(msg, fields...)
}

// Debug logs a debug message with request ID from context
func Debug(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Debug(msg, fields...)
}

// Fatal logs a fatal message with request ID from context and exits
func Fatal(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Fatal(msg, fields...)
}

// Panic logs a panic message with request ID from context and panics
func Panic(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Panic(msg, fields...)
}


