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

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNodeStageVolume(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.NodeStageVolumeRequest
		expectedResp *csi.NodeStageVolumeResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: testTargetPath,
			},
			expectedResp: &csi.NodeStageVolumeResponse{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Volume ID is missing",
			req:          &csi.NodeStageVolumeRequest{},
			expectedResp: nil,
			expectedErr:  errors.New("Volume ID missing in request"),
		},
		{
			testCaseName: "Negative: Volume target path is missing",
			req: &csi.NodeStageVolumeRequest{
				VolumeId: testVolumeID,
			},
			expectedResp: nil,
			expectedErr:  errors.New("Target path missing in request"),
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		nodeServer := nodeServer{}
		actualResp, actualErr := nodeServer.NodeStageVolume(ctx, tc.req)

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

func TestNodeUnstageVolume(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.NodeUnstageVolumeRequest
		expectedResp *csi.NodeUnstageVolumeResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: testTargetPath,
			},
			expectedResp: &csi.NodeUnstageVolumeResponse{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Volume ID is missing",
			req:          &csi.NodeUnstageVolumeRequest{},
			expectedResp: nil,
			expectedErr:  errors.New("Volume ID missing in request"),
		},
		{
			testCaseName: "Negative: Volume target path is missing",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId: testVolumeID,
			},
			expectedResp: nil,
			expectedErr:  errors.New("Target path missing in request"),
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		nodeServer := nodeServer{}
		actualResp, actualErr := nodeServer.NodeUnstageVolume(ctx, tc.req)

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

func TestNodePublishVolume(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.NodePublishVolumeRequest
		expectedResp *csi.NodePublishVolumeResponse
		expectedErr  error
	}{}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		nodeServer := nodeServer{}
		actualResp, actualErr := nodeServer.NodePublishVolume(ctx, tc.req)

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

func TestNodeUnpublishVolume(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.NodeUnpublishVolumeRequest
		mounterUtils mounterUtils.MounterUtils
		expectedResp *csi.NodeUnpublishVolumeResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   testVolumeID,
				TargetPath: testTargetPath,
			},
			mounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
				FuseUnmountFn: func(path string) error {
					return nil
				},
			}),
			expectedResp: &csi.NodeUnpublishVolumeResponse{},
			expectedErr:  nil,
		},
		// {
		// 	testCaseName: "Negative: Volume ID is missing",
		// 	req:          &csi.NodeUnstageVolumeRequest{},
		// 	expectedResp: nil,
		// 	expectedErr:  errors.New("Volume ID missing in request"),
		// },
		// {
		// 	testCaseName: "Negative: Volume target path is missing",
		// 	req: &csi.NodeUnstageVolumeRequest{
		// 		VolumeId: testVolumeID,
		// 	},
		// 	expectedResp: nil,
		// 	expectedErr:  errors.New("Target path missing in request"),
		// },
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		nodeServer := nodeServer{
			MounterUtils: tc.mounterUtils,
		}
		actualResp, actualErr := nodeServer.NodeUnpublishVolume(ctx, tc.req)

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
