/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package logger

import (
	"context"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWithRequestID(t *testing.T) {
	tests := []struct {
		name           string
		ctx            context.Context
		logger         *zap.Logger
		expectedReqID  string
		expectReqIDSet bool
	}{
		{
			name:           "Context with request ID",
			ctx:            requestid.WithRequestID(context.Background(), "test-req-123"),
			logger:         zap.NewNop(),
			expectedReqID:  "test-req-123",
			expectReqIDSet: true,
		},
		{
			name:           "Context without request ID",
			ctx:            context.Background(),
			logger:         zap.NewNop(),
			expectedReqID:  "",
			expectReqIDSet: false,
		},
		{
			name:           "Nil logger",
			ctx:            requestid.WithRequestID(context.Background(), "test-req-456"),
			logger:         nil,
			expectedReqID:  "",
			expectReqIDSet: false,
		},
		{
			name:           "Empty request ID",
			ctx:            requestid.WithRequestID(context.Background(), ""),
			logger:         zap.NewNop(),
			expectedReqID:  "",
			expectReqIDSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WithRequestID(tt.ctx, tt.logger)

			if tt.logger == nil {
				assert.Nil(t, result, "Expected nil logger when input is nil")
				return
			}

			assert.NotNil(t, result, "Expected non-nil logger")

			// Verify request ID is added by logging and checking fields
			if tt.expectReqIDSet {
				core, recorded := observer.New(zapcore.InfoLevel)
				testLogger := zap.New(core)
				resultLogger := WithRequestID(tt.ctx, testLogger)
				resultLogger.Info("test message")

				entries := recorded.All()
				assert.Len(t, entries, 1, "Expected one log entry")
				if len(entries) > 0 {
					fields := entries[0].Context
					found := false
					for _, field := range fields {
						if field.Key == "request_id" && field.String == tt.expectedReqID {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected request_id field with value %s", tt.expectedReqID)
				}
			}
		})
	}
}

func TestInfo(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	tests := []struct {
		name          string
		ctx           context.Context
		msg           string
		fields        []zap.Field
		expectedReqID string
	}{
		{
			name:          "Info with request ID",
			ctx:           requestid.WithRequestID(context.Background(), "info-req-123"),
			msg:           "test info message",
			fields:        []zap.Field{zap.String("key", "value")},
			expectedReqID: "info-req-123",
		},
		{
			name:          "Info without request ID",
			ctx:           context.Background(),
			msg:           "test info without req id",
			fields:        []zap.Field{},
			expectedReqID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorded.TakeAll() // Clear previous entries

			Info(tt.ctx, logger, tt.msg, tt.fields...)

			entries := recorded.All()
			assert.Len(t, entries, 1, "Expected one log entry")
			assert.Equal(t, tt.msg, entries[0].Message)
			assert.Equal(t, zapcore.InfoLevel, entries[0].Level)

			if tt.expectedReqID != "" {
				found := false
				for _, field := range entries[0].Context {
					if field.Key == "request_id" && field.String == tt.expectedReqID {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected request_id field")
			}
		})
	}
}

func TestError(t *testing.T) {
	core, recorded := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	tests := []struct {
		name          string
		ctx           context.Context
		msg           string
		fields        []zap.Field
		expectedReqID string
	}{
		{
			name:          "Error with request ID",
			ctx:           requestid.WithRequestID(context.Background(), "error-req-456"),
			msg:           "test error message",
			fields:        []zap.Field{zap.Error(assert.AnError)},
			expectedReqID: "error-req-456",
		},
		{
			name:          "Error without request ID",
			ctx:           context.Background(),
			msg:           "test error without req id",
			fields:        []zap.Field{},
			expectedReqID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorded.TakeAll() // Clear previous entries

			Error(tt.ctx, logger, tt.msg, tt.fields...)

			entries := recorded.All()
			assert.Len(t, entries, 1, "Expected one log entry")
			assert.Equal(t, tt.msg, entries[0].Message)
			assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)

			if tt.expectedReqID != "" {
				found := false
				for _, field := range entries[0].Context {
					if field.Key == "request_id" && field.String == tt.expectedReqID {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected request_id field")
			}
		})
	}
}

func TestWarn(t *testing.T) {
	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	tests := []struct {
		name          string
		ctx           context.Context
		msg           string
		fields        []zap.Field
		expectedReqID string
	}{
		{
			name:          "Warn with request ID",
			ctx:           requestid.WithRequestID(context.Background(), "warn-req-789"),
			msg:           "test warning message",
			fields:        []zap.Field{zap.String("warning", "details")},
			expectedReqID: "warn-req-789",
		},
		{
			name:          "Warn without request ID",
			ctx:           context.Background(),
			msg:           "test warning without req id",
			fields:        []zap.Field{},
			expectedReqID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorded.TakeAll() // Clear previous entries

			Warn(tt.ctx, logger, tt.msg, tt.fields...)

			entries := recorded.All()
			assert.Len(t, entries, 1, "Expected one log entry")
			assert.Equal(t, tt.msg, entries[0].Message)
			assert.Equal(t, zapcore.WarnLevel, entries[0].Level)

			if tt.expectedReqID != "" {
				found := false
				for _, field := range entries[0].Context {
					if field.Key == "request_id" && field.String == tt.expectedReqID {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected request_id field")
			}
		})
	}
}

func TestDebug(t *testing.T) {
	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	tests := []struct {
		name          string
		ctx           context.Context
		msg           string
		fields        []zap.Field
		expectedReqID string
	}{
		{
			name:          "Debug with request ID",
			ctx:           requestid.WithRequestID(context.Background(), "debug-req-abc"),
			msg:           "test debug message",
			fields:        []zap.Field{zap.String("debug", "info")},
			expectedReqID: "debug-req-abc",
		},
		{
			name:          "Debug without request ID",
			ctx:           context.Background(),
			msg:           "test debug without req id",
			fields:        []zap.Field{},
			expectedReqID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorded.TakeAll() // Clear previous entries

			Debug(tt.ctx, logger, tt.msg, tt.fields...)

			entries := recorded.All()
			assert.Len(t, entries, 1, "Expected one log entry")
			assert.Equal(t, tt.msg, entries[0].Message)
			assert.Equal(t, zapcore.DebugLevel, entries[0].Level)

			if tt.expectedReqID != "" {
				found := false
				for _, field := range entries[0].Context {
					if field.Key == "request_id" && field.String == tt.expectedReqID {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected request_id field")
			}
		})
	}
}

func TestFatal(t *testing.T) {
	// Fatal calls os.Exit, so we can't test it directly
	// Instead, we verify that WithRequestID works correctly and would pass the right logger
	core, _ := observer.New(zapcore.FatalLevel)
	logger := zap.New(core)

	ctx := requestid.WithRequestID(context.Background(), "fatal-req-xyz")

	// Test that WithRequestID returns a logger with request ID
	loggerWithReqID := WithRequestID(ctx, logger)
	assert.NotNil(t, loggerWithReqID)

	// We can't actually call Fatal as it would exit the test
	// The function is covered by the WithRequestID test and integration tests
}

func TestPanic(t *testing.T) {
	core, recorded := observer.New(zapcore.PanicLevel)
	logger := zap.New(core)

	ctx := requestid.WithRequestID(context.Background(), "panic-req-def")
	msg := "test panic message"

	// Panic will actually panic, so we need to recover
	defer func() {
		if r := recover(); r != nil {
			// Expected panic
			entries := recorded.All()
			assert.Len(t, entries, 1, "Expected one log entry")
			assert.Equal(t, msg, entries[0].Message)
			assert.Equal(t, zapcore.PanicLevel, entries[0].Level)

			found := false
			for _, field := range entries[0].Context {
				if field.Key == "request_id" && field.String == "panic-req-def" {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected request_id field")
		}
	}()

	Panic(ctx, logger, msg)
	t.Fatal("Expected panic but didn't occur")
}

func TestAllLogLevelsWithRequestID(t *testing.T) {
	// Test that all log levels properly include request ID
	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	ctx := requestid.WithRequestID(context.Background(), "multi-level-req")

	// Test Info
	Info(ctx, logger, "info message")
	// Test Error
	Error(ctx, logger, "error message")
	// Test Warn
	Warn(ctx, logger, "warn message")
	// Test Debug
	Debug(ctx, logger, "debug message")

	entries := recorded.All()
	assert.Len(t, entries, 4, "Expected four log entries")

	// Verify all entries have request ID
	for _, entry := range entries {
		found := false
		for _, field := range entry.Context {
			if field.Key == "request_id" && field.String == "multi-level-req" {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected request_id in %s level log", entry.Level)
	}
}
