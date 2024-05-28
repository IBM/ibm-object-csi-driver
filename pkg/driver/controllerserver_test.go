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
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ctx = context.Background()

	driverName    = "testDriver"
	driverVersion = "testDriverVersion"

	testVolumeID   = "testVolumeID"
	testVolumeName = "test-volume-name"
	testTargetPath = "test/path"
	testNodeID     = "testNodeID"
	bucketName     = "testBucket"

	testSecret = map[string]string{
		"accessKey":          "testAccessKey",
		"secretKey":          "testSecretKey",
		"locationConstraint": "test-region",
		"cosEndpoint":        "test-endpoint",
		"bucketName":         bucketName,
	}

	testEndpoint = flag.String("endpoint", "unix:/tmp/testcsi.sock", "Test CSI endpoint")
)

func TestCreateVolume(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.CreateVolumeRequest
		cosSession   s3client.ObjectStorageSessionFactory
		expectedResp *csi.CreateVolumeResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successfully created volume",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Parameters: map[string]string{},
				Secrets:    testSecret,
			},
			cosSession: &s3client.FakeCOSSessionFactory{},
			expectedResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId: testVolumeName,
					VolumeContext: map[string]string{
						"bucketName":         bucketName,
						"userProvidedBucket": "true",
					},
				},
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Positive: kpRootKeyCRN is enabled while creating volume",
			req: &csi.CreateVolumeRequest{
				Name: strings.Repeat("vol", 22),
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Parameters: map[string]string{},
				Secrets: map[string]string{
					"accessKey":          "testAccessKey",
					"secretKey":          "testSecretKey",
					"locationConstraint": "test-region",
					"cosEndpoint":        "test-endpoint",
					"kpRootKeyCRN":       "test-kpRootKeyCRN",
				},
			},
			cosSession: &s3client.FakeCOSSessionFactory{},
			expectedResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId: testVolumeName,
					VolumeContext: map[string]string{
						"userProvidedBucket": "false",
					},
				},
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Negative: Volume Name is missing",
			req: &csi.CreateVolumeRequest{
				Name: "",
			},
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: nil,
			expectedErr:  errors.New("Volume name missing in request"),
		},
		{
			testCaseName: "Negative: Volume Capabilities are missing",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
			},
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: nil,
			expectedErr:  errors.New("Volume Capabilities missing in request"),
		},
		{
			testCaseName: "Negative: Invalid Volume Capabilities",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Block{
							Block: &csi.VolumeCapability_BlockVolume{},
						},
					},
				},
			},
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: nil,
			expectedErr:  errors.New("Volume type block Volume not supported"),
		},
		{
			testCaseName: "Negative: Secret Key not provided",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Secrets: map[string]string{
					"accessKey": "testAccessKey",
				},
			},
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: nil,
			expectedErr:  errors.New("Error in getting credentials"),
		},
		{
			testCaseName: "Negative: API Key is present in secret but not service ID",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Secrets: map[string]string{
					"iamEndpoint": "testIAMEndpoint",
					"apiKey":      "testAPIKey",
				},
			},
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: nil,
			expectedErr:  errors.New("Error in getting credentials"),
		},
		{
			testCaseName: "Negative: cosEndpoint is missing",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Secrets: map[string]string{
					"accessKey": "testAccessKey",
					"secretKey": "testSecretKey",
				},
			},
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: nil,
			expectedErr:  errors.New("cosEndpoint unknown"),
		},
		{
			testCaseName: "Negative: locationConstraint is missing",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Secrets: map[string]string{
					"accessKey":   "testAccessKey",
					"secretKey":   "testSecretKey",
					"cosEndpoint": "test-endpoint",
				},
			},
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: nil,
			expectedErr:  errors.New("locationConstraint unknown"),
		},
		{
			testCaseName: "Negative: Failed to check bucket access",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Parameters: map[string]string{},
				Secrets:    testSecret,
			},
			cosSession: &s3client.FakeCOSSessionFactory{
				FailCheckBucketAccess: true,
			},
			expectedResp: nil,
			expectedErr:  errors.New("unable to access the bucket"),
		},
		{
			testCaseName: "Negative: Failed to create the bucket",
			req: &csi.CreateVolumeRequest{
				Name: testVolumeName,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
				Parameters: map[string]string{},
				Secrets:    testSecret,
			},
			cosSession: &s3client.FakeCOSSessionFactory{
				FailCheckBucketAccess: true,
				FailCreateBucket:      true,
			},
			expectedResp: nil,
			expectedErr:  errors.New("unable to create the bucket"),
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		controllerServer := &controllerServer{
			cosSession: tc.cosSession,
		}
		actualResp, actualErr := controllerServer.CreateVolume(ctx, tc.req)

		if tc.expectedErr != nil {
			assert.Error(t, actualErr)
			assert.Contains(t, actualErr.Error(), tc.expectedErr.Error())
		} else {
			assert.NoError(t, actualErr)
		}

		if len(tc.req.Name) > 63 {
			tc.expectedResp.Volume.VolumeId = actualResp.Volume.VolumeId
		}
		if actualResp != nil && strings.Contains(actualResp.Volume.VolumeContext["bucketName"], actualResp.Volume.VolumeId) {
			tc.expectedResp.Volume.VolumeContext["bucketName"] = actualResp.Volume.VolumeContext["bucketName"]
		}

		if !reflect.DeepEqual(tc.expectedResp, actualResp) {
			t.Errorf("Expected %v but got %v", tc.expectedResp, actualResp)
		}
	}
}

func TestDeleteVolume(t *testing.T) {
	testCases := []struct {
		testCaseName     string
		req              *csi.DeleteVolumeRequest
		driverStatsUtils utils.StatsUtils
		cosSession       s3client.ObjectStorageSessionFactory
		expectedResp     *csi.DeleteVolumeResponse
		expectedErr      error
	}{
		{
			testCaseName: "Positive: Successfully deleted volume",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolumeID,
				Secrets:  testSecret,
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				BucketToDeleteFn: func(volumeID string) (string, error) {
					return bucketName, nil
				},
			}),
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: &csi.DeleteVolumeResponse{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Volume ID is missing",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "",
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{}),
			cosSession:       &s3client.FakeCOSSessionFactory{},
			expectedResp:     nil,
			expectedErr:      errors.New("Volume ID missing"),
		},
		{
			testCaseName: "Negative: Access Key not provided",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolumeID,
				Secrets:  map[string]string{},
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{}),
			cosSession:       &s3client.FakeCOSSessionFactory{},
			expectedResp:     nil,
			expectedErr:      errors.New("cannot get credentials"),
		},
		{
			testCaseName: "Incomplete: Can't delete bucket",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolumeID,
				Secrets:  testSecret,
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				BucketToDeleteFn: func(volumeID string) (string, error) {
					return bucketName, nil
				},
			}),
			cosSession: &s3client.FakeCOSSessionFactory{
				FailDeleteBucket: true,
			},
			expectedResp: &csi.DeleteVolumeResponse{},
			expectedErr:  nil,
		},
		{
			testCaseName: "Negative: Failed to get bucket to delete",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolumeID,
				Secrets:  testSecret,
			},
			driverStatsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				BucketToDeleteFn: func(volumeID string) (string, error) {
					return "", errors.New("failed to get bucket to delete")
				},
			}),
			cosSession:   &s3client.FakeCOSSessionFactory{},
			expectedResp: &csi.DeleteVolumeResponse{},
			expectedErr:  nil,
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		lgr, teardown := GetTestLogger(t)
		defer teardown()

		controllerServer := &controllerServer{
			Stats:      tc.driverStatsUtils,
			cosSession: tc.cosSession,
			Logger:     lgr,
		}
		actualResp, actualErr := controllerServer.DeleteVolume(ctx, tc.req)

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

func TestControllerPublishVolume(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ControllerPublishVolume(ctx, &csi.ControllerPublishVolumeRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestControllerUnpublishVolume(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ControllerUnpublishVolume(ctx, &csi.ControllerUnpublishVolumeRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestValidateVolumeCapabilities(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.ValidateVolumeCapabilitiesRequest
		expectedResp *csi.ValidateVolumeCapabilitiesResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successfully validated volume Capabilities",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: testVolumeID,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: volumeCapabilities[0],
						},
					},
				},
			},
			expectedResp: &csi.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeCapabilities: []*csi.VolumeCapability{
						{

							AccessMode: &csi.VolumeCapability_AccessMode{
								Mode: volumeCapabilities[0],
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Negative: Volume ID is missing",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: "",
			},
			expectedResp: nil,
			expectedErr:  errors.New("Volume ID missing in request"),
		},
		{
			testCaseName: "Negative: Volume capabilities are missing",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: testVolumeID,
			},
			expectedResp: nil,
			expectedErr:  errors.New("Volume capabilities missing in request"),
		},
		{
			testCaseName: "Negative: Invalid Volume Capabilities",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: testVolumeID,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
				},
			},
			expectedResp: &csi.ValidateVolumeCapabilitiesResponse{},
			expectedErr:  nil,
		},
	}

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ValidateVolumeCapabilities(ctx, tc.req)

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

func TestListVolumes(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ListVolumes(ctx, &csi.ListVolumesRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestGetCapacity(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.GetCapacity(ctx, &csi.GetCapacityRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestControllerGetCapabilities(t *testing.T) {
	testCases := []struct {
		testCaseName string
		req          *csi.ControllerGetCapabilitiesRequest
		expectedResp *csi.ControllerGetCapabilitiesResponse
		expectedErr  error
	}{
		{
			testCaseName: "Positive: Successfully get controller capabilities",
			req:          &csi.ControllerGetCapabilitiesRequest{},
			expectedResp: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: controllerCapabilities[0],
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

		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ControllerGetCapabilities(ctx, tc.req)

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

func TestCreateSnapshot(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.CreateSnapshot(ctx, &csi.CreateSnapshotRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestDeleteSnapshot(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.DeleteSnapshot(ctx, &csi.DeleteSnapshotRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestListSnapshots(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ListSnapshots(ctx, &csi.ListSnapshotsRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestControllerExpandVolume(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ControllerExpandVolume(ctx, &csi.ControllerExpandVolumeRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestControllerGetVolume(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ControllerGetVolume(ctx, &csi.ControllerGetVolumeRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}

func TestControllerModifyVolume(t *testing.T) {
	t.Run("UnImplemented Method", func(t *testing.T) {
		controllerServer := &controllerServer{}
		actualResp, actualErr := controllerServer.ControllerModifyVolume(ctx, &csi.ControllerModifyVolumeRequest{})
		assert.Nil(t, actualResp)
		assert.Error(t, actualErr)
		assert.Contains(t, actualErr.Error(), status.Error(codes.Unimplemented, "").Error())
	})
}
