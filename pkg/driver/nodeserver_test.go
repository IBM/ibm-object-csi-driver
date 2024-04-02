/**
 * Copyright 2021 IBM Corp.
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

// Package driver ...
package driver

import (
	"errors"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultVolumeID = "csiprovidervolumeid"
const defaultTargetPath = "/mnt/test"
const defaultStagingPath = "/staging"

var (
	unmountSuccess            = func(target string) error { return nil }
	unmountFailure            = func(target string) error { return errors.New("") }
	checkMountFailure         = func(target string) (bool, error) { return false, errors.New("") }
	checkMountFailureNotMount = func(target string) (bool, error) { return false, nil }
)

func TestNodePublishVolume(t *testing.T) {
	// newmounter = fakemounter.NewMounter
	testCases := []struct {
		name       string
		req        *csi.NodePublishVolumeRequest
		expErrCode codes.Code
	}{
		{
			name: "Valid request",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          defaultVolumeID,
				TargetPath:        defaultTargetPath,
				StagingTargetPath: defaultStagingPath,
				Readonly:          false,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.OK,
		},
		{
			name: "Empty volume ID",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "",
				TargetPath:        defaultTargetPath,
				StagingTargetPath: defaultStagingPath,
				Readonly:          false,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "Empty staging target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "testvolumeid",
				TargetPath:        defaultTargetPath,
				StagingTargetPath: "",
				Readonly:          false,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "Empty target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "testvolumeid",
				TargetPath:        "",
				StagingTargetPath: defaultTargetPath,
				Readonly:          false,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "Empty volume capabilities",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "testvolumeid",
				TargetPath:        defaultTargetPath,
				StagingTargetPath: defaultStagingPath,
				Readonly:          false,
				VolumeCapability:  nil,
			},
			expErrCode: codes.InvalidArgument,
		},
	}
	icDriver := inits3Driver(t)

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := icDriver.ns.NodePublishVolume(context.Background(), tc.req)
		if err != nil {
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", err)
			}
			if serverError.Code() != tc.expErrCode {
				t.Fatalf("Expected error code: %v, got: %v. err : %v", tc.expErrCode, serverError.Code(), err)
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error: %v, got no error", tc.expErrCode)
		}
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	fuseunmount = unmountSuccess
	testCases := []struct {
		name       string
		req        *csi.NodeUnpublishVolumeRequest
		expErrCode codes.Code
	}{
		{
			name: "Valid request",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   defaultVolumeID,
				TargetPath: defaultTargetPath,
			},
			expErrCode: codes.OK,
		},
		{
			name: "Empty volume ID",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   "",
				TargetPath: defaultTargetPath,
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "Empty target path",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   defaultVolumeID,
				TargetPath: "",
			},
			expErrCode: codes.InvalidArgument,
		},
	}

	icDriver := inits3Driver(t)

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := icDriver.ns.NodeUnpublishVolume(context.Background(), tc.req)
		if err != nil {
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", err)
			}
			if serverError.Code() != tc.expErrCode {
				t.Fatalf("Expected error code: %v, got: %v. err : %v", tc.expErrCode, serverError.Code(), err)
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error: %v, got no error", tc.expErrCode)
		}
	}
}

func TestNodeStageVolume(t *testing.T) {
	volumeID := "newstagevolumeID"
	testCases := []struct {
		name       string
		req        *csi.NodeStageVolumeRequest
		expErrCode codes.Code
	}{
		{
			name: "Valid request",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          volumeID,
				StagingTargetPath: defaultStagingPath,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.OK,
		},
		{
			name: "Empty volume ID",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "",
				StagingTargetPath: defaultStagingPath,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "Empty Stage target path",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          volumeID,
				StagingTargetPath: "",
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "Empty volume capabilities",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          volumeID,
				StagingTargetPath: defaultTargetPath,
				VolumeCapability:  nil,
			},
			expErrCode: codes.InvalidArgument,
		},
	}

	icDriver := inits3Driver(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := icDriver.ns.NodeStageVolume(context.Background(), tc.req)
		if err != nil {
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", err)
			}
			if serverError.Code() != tc.expErrCode {
				t.Fatalf("Expected error code: %v, got: %v. err : %v", tc.expErrCode, serverError.Code(), err)
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error: %v, got no error", tc.expErrCode)
		}
	}
}

func TestNodeUnstageVolume(t *testing.T) {
	testCases := []struct {
		name       string
		req        *csi.NodeUnstageVolumeRequest
		expErrCode codes.Code
	}{
		{
			name: "Valid request",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          defaultVolumeID,
				StagingTargetPath: defaultTargetPath,
			},
			expErrCode: codes.OK,
		},
		{
			name: "Empty volume ID",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "",
				StagingTargetPath: defaultStagingPath,
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "Empty target path",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          defaultVolumeID,
				StagingTargetPath: "",
			},
			expErrCode: codes.InvalidArgument,
		},
	}

	icDriver := inits3Driver(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := icDriver.ns.NodeUnstageVolume(context.Background(), tc.req)
		if err != nil {
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", err)
			}
			if serverError.Code() != tc.expErrCode {
				t.Fatalf("Expected error code: %v, got: %v. err : %v", tc.expErrCode, serverError.Code(), err)
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error: %v, got no error", tc.expErrCode)
		}
	}
}

func TestNodeGetCapabilities(t *testing.T) {
	req := &csi.NodeGetCapabilitiesRequest{}

	icDriver := inits3Driver(t)
	_, err := icDriver.ns.NodeGetCapabilities(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpedted error: %v", err)
	}
}

func TestNodeExpandVolume(t *testing.T) {
	icDriver := inits3Driver(t)
	_, err := icDriver.ns.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{})
	assert.NotNil(t, err)
}

func TestNodeUnpublishVolumeUnMountFail(t *testing.T) {
	fuseunmount = unmountFailure
	testCases := []struct {
		name       string
		req        *csi.NodeUnpublishVolumeRequest
		expErrCode codes.Code
	}{
		{
			name: "Unmount failure",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   defaultVolumeID,
				TargetPath: defaultStagingPath,
			},
			expErrCode: codes.Internal,
		},
	}

	icDriver := inits3Driver(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := icDriver.ns.NodeUnpublishVolume(context.Background(), tc.req)
		if err != nil {
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", err)
			}
			if serverError.Code() != tc.expErrCode {
				t.Fatalf("Expected error code: %v, got: %v. err : %v", tc.expErrCode, serverError.Code(), err)
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error: %v, got no error", tc.expErrCode)
		}
	}
}

func TestNodePublishVolumeCheckMountFail(t *testing.T) {
	checkMountpoint = checkMountFailure
	testCases := []struct {
		name       string
		req        *csi.NodePublishVolumeRequest
		expErrCode codes.Code
	}{
		{
			name: "checkMount failure",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          defaultVolumeID,
				TargetPath:        defaultTargetPath,
				StagingTargetPath: defaultStagingPath,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: codes.Internal,
		},
	}

	icDriver := inits3Driver(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := icDriver.ns.NodePublishVolume(context.Background(), tc.req)
		if err != nil {
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", err)
			}
			if serverError.Code() != tc.expErrCode {
				t.Fatalf("Expected error code: %v, got: %v. err : %v", tc.expErrCode, serverError.Code(), err)
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error: %v, got no error", tc.expErrCode)
		}
	}
}

func TestNodePublishVolumeCheckMountIsNotMountFail(t *testing.T) {
	checkMountpoint = checkMountFailureNotMount
	testCases := []struct {
		name       string
		req        *csi.NodePublishVolumeRequest
		expErrCode error
	}{
		{
			name: "checkMount failure - not a mount point",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          defaultVolumeID,
				TargetPath:        defaultTargetPath,
				StagingTargetPath: defaultStagingPath,
				VolumeCapability:  stdVolCap[0],
			},
			expErrCode: nil,
		},
	}

	icDriver := inits3Driver(t)
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		_, err := icDriver.ns.NodePublishVolume(context.Background(), tc.req)
		assert.Nil(t, err)
	}
}
