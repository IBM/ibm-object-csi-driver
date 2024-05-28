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
	"errors"
	"flag"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestNonBlockingGRPCServer(t *testing.T) {
	t.Run("Positive", func(t *testing.T) {
		lgr, teardown := GetTestLogger(t)
		defer teardown()

		s := NewNonBlockingGRPCServer("controller", lgr)
		nonBlockingServer, ok := s.(*nonBlockingGRPCServer)
		assert.Equal(t, true, ok)

		listener, err := nonBlockingServer.Setup(*testEndpoint, &identityServer{}, &controllerServer{}, &nodeServer{})
		assert.NoError(t, err)
		assert.NotNil(t, listener)

		nonBlockingServer.Wait()
		nonBlockingServer.Stop()
		nonBlockingServer.ForceStop()
	})
}

func TestSetup(t *testing.T) {
	testCases := []struct {
		testCaseName string
		endpoint     *string
		mode         string
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			endpoint:     testEndpoint,
			mode:         "controller",
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: TCP Scheme",
			endpoint:     flag.String("tcpendpoint", "tcp:/tmp/testtcpcsi.sock", "Test CSI endpoint"),
			mode:         "node",
			expectedErr:  nil,
		},
		{
			testCaseName: "Positive: controller-node mode",
			endpoint:     testEndpoint,
			mode:         "controller-node",
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Wrong endpoint format",
			endpoint:     flag.String("wrongendpoint", "---:/tmp/testcsi.sock", "Test CSI endpoint"),
			mode:         "controller",
			expectedErr:  errors.New("first path segment in URL cannot contain colon"),
		},
		{
			testCaseName: "Negative: Wrong Scheme",
			endpoint:     flag.String("wrongschemaendpoint", "wrong-scheme:/tmp/testcsi.sock", "Test CSI endpoint"),
			mode:         "controller",
			expectedErr:  errors.New("endpoint scheme not supported"),
		},
		{
			testCaseName: "Negative: Wrong address",
			endpoint:     flag.String("wrongaddressendpoint", "unix:443", "Test CSI endpoint"),
			mode:         "controller",
			// expectedErr:  errors.New("failed to listen GRPC server"),
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		lgr, teardown := GetTestLogger(t)
		defer teardown()

		server := nonBlockingGRPCServer{
			mode:   tc.mode,
			logger: lgr,
		}
		_, actualErr := server.Setup(*tc.endpoint, &identityServer{}, &controllerServer{}, &nodeServer{})

		if tc.expectedErr != nil {
			assert.Error(t, actualErr)
			assert.Contains(t, actualErr.Error(), tc.expectedErr.Error())
		} else {
			assert.NoError(t, actualErr)
		}
	}
}

func TestLogGRPC(t *testing.T) {
	testCases := []struct {
		testCaseName string
		handler      grpc.UnaryHandler
		expectedResp interface{}
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			handler: func(ctx context.Context, req any) (any, error) {
				return zap.Logger{}, nil
			},
			expectedResp: zap.Logger{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Error occurred",
			handler: func(ctx context.Context, req any) (any, error) {
				return nil, errors.New("failed")
			},
			expectedResp: nil,
			expectedErr:  errors.New("failed"),
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		actualResp, actualErr := logGRPC(ctx, nil, &grpc.UnaryServerInfo{}, tc.handler)

		if tc.expectedErr != nil {
			assert.Error(t, actualErr)
			assert.Contains(t, actualErr.Error(), tc.expectedErr.Error())
		} else {
			assert.NoError(t, actualErr)
		}

		if !reflect.DeepEqual(tc.expectedResp, actualResp) {
			t.Errorf("Expected %v but got %v", tc.expectedResp, actualResp)
		}
	}
}
