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
	"reflect"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetPluginInfo(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.GetPluginInfoRequest
		s3Driver     *S3Driver
		expectedResp *csi.GetPluginInfoResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req:          &csi.GetPluginInfoRequest{},
			s3Driver: &S3Driver{
				name:    driverName,
				version: driverVersion,
			},
			expectedResp: &csi.GetPluginInfoResponse{
				Name:          driverName,
				VendorVersion: driverVersion,
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Negative: Driver not configuraed",
			req:          &csi.GetPluginInfoRequest{},
			s3Driver:     nil,
			expectedResp: nil,
			expectedErr:  errors.New("Driver not configured"),
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		identityServer := &identityServer{
			S3Driver: tc.s3Driver,
		}
		actualResp, actualErr := identityServer.GetPluginInfo(ctx, tc.req)

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

func TestGetPluginCapabilities(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.GetPluginCapabilitiesRequest
		expectedResp *csi.GetPluginCapabilitiesResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req:          &csi.GetPluginCapabilitiesRequest{},
			expectedResp: &csi.GetPluginCapabilitiesResponse{
				Capabilities: []*csi.PluginCapability{
					{
						Type: &csi.PluginCapability_Service_{
							Service: &csi.PluginCapability_Service{
								Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
							},
						},
					},
					{
						Type: &csi.PluginCapability_Service_{
							Service: &csi.PluginCapability_Service{
								Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		identityServer := &identityServer{}
		actualResp, actualErr := identityServer.GetPluginCapabilities(ctx, tc.req)

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

func TestProbe(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.ProbeRequest
		expectedResp *csi.ProbeResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req:          &csi.ProbeRequest{},
			expectedResp: &csi.ProbeResponse{},
			expectedErr:  nil,
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		identityServer := &identityServer{}
		actualResp, actualErr := identityServer.Probe(ctx, tc.req)

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
