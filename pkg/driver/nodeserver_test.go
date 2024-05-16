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

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
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
		{
			testCaseName: "Negative: Volume ID is missing",
			req:          &csi.NodeUnpublishVolumeRequest{},
			expectedResp: nil,
			expectedErr:  errors.New("Volume ID missing in request"),
		},
		{
			testCaseName: "Negative: Volume target path is missing",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId: testVolumeID,
			},
			expectedResp: nil,
			expectedErr:  errors.New("Target path missing in request"),
		},
		{
			testCaseName: "Negative: Unmount failed",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   testVolumeID,
				TargetPath: testTargetPath,
			},
			mounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
				FuseUnmountFn: func(path string) error {
					return errors.New("cannot force unmount")
				},
			}),
			expectedResp: nil,
			expectedErr:  errors.New("cannot force unmount"),
		},
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

func TestNodeGetVolumeStats(t *testing.T) {
	testCases := []struct {
		testCaseName     string
		req              *csi.NodeGetVolumeStatsRequest
		driverStatsUtils utils.StatsUtils
		expectedResp     *csi.NodeGetVolumeStatsResponse
		expectedErr      error
	}{
		{
			testCaseName: "Positive: Successful",
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   testVolumeID,
				VolumePath: testTargetPath,
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				FSInfoFn: func(path string) (int64, int64, int64, int64, int64, int64, error) {
					return 1, 1, 1, 1, 1, 1, nil
				},
				GetBucketUsageFn: func(volumeID string) (int64, resource.Quantity, error) {
					return 1, resource.Quantity{}, nil
				},
			}),
			expectedResp: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{
						Available: -1,
						Used:      1,
						Unit:      csi.VolumeUsage_BYTES,
					},
					{
						Available: 1,
						Total:     1,
						Used:      1,
						Unit:      csi.VolumeUsage_INODES,
					},
				},
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Negative: Volume ID is missing",
			req:          &csi.NodeGetVolumeStatsRequest{},
			expectedResp: nil,
			expectedErr:  errors.New("Volume ID missing in request"),
		},
		{
			testCaseName: "Negative: Volume path is missing",
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId: testVolumeID,
			},
			expectedResp: nil,
			expectedErr:  errors.New("Path Doesn't exist"),
		},
		{
			testCaseName: "Negative: Failed to getg volume stats",
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   testVolumeID,
				VolumePath: testTargetPath,
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				FSInfoFn: func(path string) (int64, int64, int64, int64, int64, int64, error) {
					return 0, 0, 0, 0, 0, 0, errors.New("transpoint endpoint is not connected")
				},
			}),
			expectedResp: &csi.NodeGetVolumeStatsResponse{
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: true,
					Message:  "transpoint endpoint is not connected",
				},
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Negative: Failed to get Bucket Usage",
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   testVolumeID,
				VolumePath: testTargetPath,
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				FSInfoFn: func(path string) (int64, int64, int64, int64, int64, int64, error) {
					return 1, 1, 1, 1, 1, 1, nil
				},
				GetBucketUsageFn: func(volumeID string) (int64, resource.Quantity, error) {
					return 0, resource.Quantity{}, errors.New("failed to get bucket usage")
				},
			}),
			expectedResp: nil,
			expectedErr:  errors.New("failed to get bucket usage"),
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		nodeServer := nodeServer{
			Stats: tc.driverStatsUtils,
		}
		actualResp, actualErr := nodeServer.NodeGetVolumeStats(ctx, tc.req)

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

func TestNodeExpandVolume(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		nodeServer := nodeServer{}
		actualResp, actualErr := nodeServer.NodeExpandVolume(ctx, &csi.NodeExpandVolumeRequest{})
		assert.Equal(t, &csi.NodeExpandVolumeResponse{}, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestNodeGetCapabilities(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.NodeGetCapabilitiesRequest
		expectedResp *csi.NodeGetCapabilitiesResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req:          &csi.NodeGetCapabilitiesRequest{},
			expectedResp: &csi.NodeGetCapabilitiesResponse{
				Capabilities: []*csi.NodeServiceCapability{
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: nodeServerCapabilities[0],
							},
						},
					},
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: nodeServerCapabilities[1],
							},
						},
					},
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: nodeServerCapabilities[2],
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

		nodeServer := nodeServer{}
		actualResp, actualErr := nodeServer.NodeGetCapabilities(ctx, tc.req)

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

func TestNodeGetInfo(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.NodeGetInfoRequest
		expectedResp *csi.NodeGetInfoResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successful",
			req:          &csi.NodeGetInfoRequest{},
			expectedResp: &csi.NodeGetInfoResponse{
				NodeId:             testNodeID,
				MaxVolumesPerNode:  constants.DefaultVolumesPerNode,
				AccessibleTopology: &csi.Topology{},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		nodeServer := nodeServer{
			NodeID: testNodeID,
		}
		actualResp, actualErr := nodeServer.NodeGetInfo(ctx, tc.req)

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
