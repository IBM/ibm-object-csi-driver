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
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewConsoleLogger creates a new zap logger with console encoding
// This provides human-readable logs with structured fields
func NewConsoleLogger(serviceName string) (*zap.Logger, error) {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

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
// This is useful for init() functions where error handling is limited
func NewConsoleLoggerOrNop(serviceName string) *zap.Logger {
	logger, err := NewConsoleLogger(serviceName)
	if err != nil {
		return zap.NewNop()
	}
	return logger
}
