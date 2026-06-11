/**
 * Copyright 2024 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

func TestUnaryServerInterceptor(t *testing.T) {
	testCases := []struct {
		name            string
		ctx             context.Context
		handler         grpc.UnaryHandler
		expectRequestID bool
		expectError     bool
		expectedErrMsg  string
	}{
		{
			name: "Generates new request ID when context is empty",
			ctx:  context.Background(),
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				// Verify request ID exists in context
				reqID := requestid.FromContext(ctx)
				assert.NotEmpty(t, reqID, "Request ID should be present in context")

				// Verify it's a valid UUID
				_, err := uuid.Parse(reqID)
				assert.NoError(t, err, "Request ID should be a valid UUID")

				return "success", nil
			},
			expectRequestID: true,
			expectError:     false,
		},
		{
			name: "Preserves existing request ID in context",
			ctx:  requestid.WithRequestID(context.Background(), "existing-request-id-123"),
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				// Verify the existing request ID is preserved
				reqID := requestid.FromContext(ctx)
				assert.Equal(t, "existing-request-id-123", reqID, "Existing request ID should be preserved")
				return "success", nil
			},
			expectRequestID: true,
			expectError:     false,
		},
		{
			name: "Logs request ID on successful completion",
			ctx:  context.Background(),
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return "success", nil
			},
			expectRequestID: true,
			expectError:     false,
		},
		{
			name: "Logs request ID on error",
			ctx:  context.Background(),
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, errors.New("test error")
			},
			expectRequestID: true,
			expectError:     true,
			expectedErrMsg:  "test error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create an observed logger to capture log entries
			core, recorded := observer.New(zapcore.InfoLevel)
			logger := zap.New(core)

			// Create the interceptor
			interceptor := UnaryServerInterceptor(logger)

			// Create mock server info
			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/TestMethod",
			}

			// Call the interceptor
			resp, err := interceptor(tc.ctx, nil, info, tc.handler)

			// Verify error handling
			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}

			// Verify logs contain request ID
			if tc.expectRequestID {
				logs := recorded.All()
				assert.NotEmpty(t, logs, "Should have log entries")

				// Check that logs contain request_id field
				foundRequestID := false
				var capturedRequestID string
				for _, log := range logs {
					for _, field := range log.Context {
						if field.Key == "request_id" {
							foundRequestID = true
							capturedRequestID = field.String

							// Verify it's not empty
							assert.NotEmpty(t, capturedRequestID, "Request ID in logs should not be empty")

							// If we had an existing request ID, verify it matches
							if existingID := requestid.FromContext(tc.ctx); existingID != "" {
								assert.Equal(t, existingID, capturedRequestID, "Logged request ID should match context")
							}
							break
						}
					}
				}
				assert.True(t, foundRequestID, "Logs should contain request_id field")

				// Verify all logs for this request have the same request ID
				requestIDs := make(map[string]bool)
				for _, log := range logs {
					for _, field := range log.Context {
						if field.Key == "request_id" {
							requestIDs[field.String] = true
						}
					}
				}
				assert.Equal(t, 1, len(requestIDs), "All logs should have the same request ID")
			}

			// Verify log messages
			logs := recorded.All()
			if tc.expectError {
				// Should have "started" and "failed" logs
				var hasStarted, hasFailed bool
				for _, log := range logs {
					if log.Message == "gRPC request started" {
						hasStarted = true
					}
					if log.Message == "gRPC request failed" {
						hasFailed = true
					}
				}
				assert.True(t, hasStarted, "Should log request started")
				assert.True(t, hasFailed, "Should log request failed")
			} else {
				// Should have "started" and "completed" logs
				var hasStarted, hasCompleted bool
				for _, log := range logs {
					if log.Message == "gRPC request started" {
						hasStarted = true
					}
					if log.Message == "gRPC request completed" {
						hasCompleted = true
					}
				}
				assert.True(t, hasStarted, "Should log request started")
				assert.True(t, hasCompleted, "Should log request completed")
			}
		})
	}
}

func TestUnaryServerInterceptor_RequestIDPropagation(t *testing.T) {
	// Create an observed logger
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	// Create the interceptor
	interceptor := UnaryServerInterceptor(logger)

	// Track the request ID through nested calls
	var capturedRequestID string
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedRequestID = requestid.FromContext(ctx)

		// Simulate nested operation that uses the same context
		nestedFunc := func(ctx context.Context) string {
			return requestid.FromContext(ctx)
		}

		nestedRequestID := nestedFunc(ctx)
		assert.Equal(t, capturedRequestID, nestedRequestID, "Request ID should propagate to nested calls")

		return "success", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	// Call the interceptor
	_, err := interceptor(context.Background(), nil, info, handler)
	assert.NoError(t, err)

	// Verify request ID was captured
	assert.NotEmpty(t, capturedRequestID, "Request ID should be captured from context")

	// Verify it's a valid UUID
	_, parseErr := uuid.Parse(capturedRequestID)
	assert.NoError(t, parseErr, "Request ID should be a valid UUID")

	// Verify logs contain the same request ID
	logs := recorded.All()
	for _, log := range logs {
		for _, field := range log.Context {
			if field.Key == "request_id" {
				assert.Equal(t, capturedRequestID, field.String, "All logs should have the same request ID")
			}
		}
	}
}

func TestUnaryServerInterceptor_DurationLogging(t *testing.T) {
	// Create an observed logger
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	// Create the interceptor
	interceptor := UnaryServerInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	// Call the interceptor
	_, err := interceptor(context.Background(), nil, info, handler)
	assert.NoError(t, err)

	// Verify duration is logged
	logs := recorded.All()
	foundDuration := false
	for _, log := range logs {
		if log.Message == "gRPC request completed" {
			for _, field := range log.Context {
				if field.Key == "duration" {
					foundDuration = true
					assert.Greater(t, field.Integer, int64(0), "Duration should be greater than 0")
					break
				}
			}
		}
	}
	assert.True(t, foundDuration, "Should log request duration")
}
