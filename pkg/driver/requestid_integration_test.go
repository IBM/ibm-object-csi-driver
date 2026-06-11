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
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

// TestRequestIDEndToEndFlow tests the complete request ID flow from
// interceptor through CSI handlers to verify proper propagation
func TestRequestIDEndToEndFlow(t *testing.T) {
	// Create an observed logger to capture all log entries
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	// Create identity server
	s3Driver := &S3Driver{
		name:    "test-driver",
		version: "1.0.0",
		logger:  logger,
	}
	identityServer := &identityServer{
		S3Driver: s3Driver,
	}

	// Create the interceptor
	interceptor := UnaryServerInterceptor(logger)

	// Test GetPluginInfo through the interceptor
	t.Run("GetPluginInfo with request ID propagation", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs

		req := &csi.GetPluginInfoRequest{}
		ctx := context.Background()

		// Handler that will be called by interceptor
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			// Verify request ID exists in context
			reqID := requestid.FromContext(ctx)
			assert.NotEmpty(t, reqID, "Request ID should be present in context")

			// Call the actual CSI method
			return identityServer.GetPluginInfo(ctx, req.(*csi.GetPluginInfoRequest))
		}

		// Call through interceptor
		resp, err := interceptor(ctx, req, &grpc.UnaryServerInfo{
			FullMethod: "/csi.v1.Identity/GetPluginInfo",
		}, handler)

		// Verify response
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		pluginResp, ok := resp.(*csi.GetPluginInfoResponse)
		assert.True(t, ok)
		assert.Equal(t, "test-driver", pluginResp.Name)

		// Verify logs contain request ID
		logs := recorded.All()
		assert.NotEmpty(t, logs, "Should have log entries")

		// Extract request ID from logs
		var requestIDs []string
		for _, log := range logs {
			for _, field := range log.Context {
				if field.Key == "request_id" {
					requestIDs = append(requestIDs, field.String)
				}
			}
		}

		// Verify all logs have the same request ID
		assert.NotEmpty(t, requestIDs, "Should have request IDs in logs")
		firstReqID := requestIDs[0]
		assert.NotEmpty(t, firstReqID, "Request ID should not be empty")

		for _, reqID := range requestIDs {
			assert.Equal(t, firstReqID, reqID, "All logs should have the same request ID")
		}

		// Verify we have both start and completion logs
		var hasStartLog, hasCompletionLog bool
		for _, log := range logs {
			if log.Message == "gRPC request started" {
				hasStartLog = true
			}
			if log.Message == "gRPC request completed" {
				hasCompletionLog = true
			}
		}
		assert.True(t, hasStartLog, "Should have request started log")
		assert.True(t, hasCompletionLog, "Should have request completed log")
	})

	t.Run("Probe with existing request ID", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs

		req := &csi.ProbeRequest{}
		existingReqID := "existing-probe-request-id"
		ctx := requestid.WithRequestID(context.Background(), existingReqID)

		// Handler that will be called by interceptor
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			// Verify the existing request ID is preserved
			reqID := requestid.FromContext(ctx)
			assert.Equal(t, existingReqID, reqID, "Should preserve existing request ID")

			// Call the actual CSI method
			return identityServer.Probe(ctx, req.(*csi.ProbeRequest))
		}

		// Call through interceptor
		resp, err := interceptor(ctx, req, &grpc.UnaryServerInfo{
			FullMethod: "/csi.v1.Identity/Probe",
		}, handler)

		// Verify response
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify logs contain the existing request ID
		logs := recorded.All()
		for _, log := range logs {
			for _, field := range log.Context {
				if field.Key == "request_id" {
					assert.Equal(t, existingReqID, field.String, "Logs should contain existing request ID")
				}
			}
		}
	})
}

// TestRequestIDPropagationToLogger tests that request ID properly
// propagates to logger instances
func TestRequestIDPropagationToLogger(t *testing.T) {
	// Create an observed logger at DEBUG level to capture all logs
	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	testReqID := "test-logger-request-id"
	ctx := requestid.WithRequestID(context.Background(), testReqID)

	// Create identity server
	s3Driver := &S3Driver{
		name:    "test-driver",
		version: "1.0.0",
		logger:  logger,
	}
	identityServer := &identityServer{
		S3Driver: s3Driver,
	}

	// Call GetPluginInfo which should log with request ID
	req := &csi.GetPluginInfoRequest{}
	resp, err := identityServer.GetPluginInfo(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify logs contain the request ID
	logs := recorded.All()
	assert.NotEmpty(t, logs, "Should have log entries")

	foundRequestID := false
	for _, log := range logs {
		for _, field := range log.Context {
			if field.Key == "request_id" && field.String == testReqID {
				foundRequestID = true
				break
			}
		}
	}
	assert.True(t, foundRequestID, "Logs should contain the request ID")
}

// TestRequestIDInMultipleHandlers tests request ID propagation
// across different CSI handler types
func TestRequestIDInMultipleHandlers(t *testing.T) {
	// Create an observed logger
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	testReqID := "multi-handler-request-id"
	ctx := requestid.WithRequestID(context.Background(), testReqID)

	// Create servers
	s3Driver := &S3Driver{
		name:    "test-driver",
		version: "1.0.0",
		logger:  logger,
	}

	identityServer := &identityServer{S3Driver: s3Driver}
	nodeServer := &nodeServer{S3Driver: s3Driver}

	testCases := []struct {
		name    string
		handler func() error
	}{
		{
			name: "IdentityServer.GetPluginInfo",
			handler: func() error {
				_, err := identityServer.GetPluginInfo(ctx, &csi.GetPluginInfoRequest{})
				return err
			},
		},
		{
			name: "IdentityServer.GetPluginCapabilities",
			handler: func() error {
				_, err := identityServer.GetPluginCapabilities(ctx, &csi.GetPluginCapabilitiesRequest{})
				return err
			},
		},
		{
			name: "IdentityServer.Probe",
			handler: func() error {
				_, err := identityServer.Probe(ctx, &csi.ProbeRequest{})
				return err
			},
		},
		{
			name: "NodeServer.NodeGetCapabilities",
			handler: func() error {
				_, err := nodeServer.NodeGetCapabilities(ctx, &csi.NodeGetCapabilitiesRequest{})
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorded.TakeAll() // Clear previous logs

			err := tc.handler()
			assert.NoError(t, err)

			// Verify logs contain the request ID
			logs := recorded.All()
			if len(logs) > 0 {
				foundRequestID := false
				for _, log := range logs {
					for _, field := range log.Context {
						if field.Key == "request_id" && field.String == testReqID {
							foundRequestID = true
							break
						}
					}
				}
				assert.True(t, foundRequestID, "Logs should contain the request ID for "+tc.name)
			}
		})
	}
}
