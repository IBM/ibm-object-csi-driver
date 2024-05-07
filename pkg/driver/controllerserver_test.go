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
	"reflect"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

var (
	// Define "normal" parameters
	volCaps = []*csi.VolumeCapability{
		{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			},
		},
	}
)

func TestControllerPublishVolume(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.ControllerPublishVolumeRequest
		expResponse *csi.ControllerPublishVolumeResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success attachment",
			req:         &csi.ControllerPublishVolumeRequest{VolumeId: "volumeid", NodeId: "nodeid"},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.ControllerPublishVolume(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestControllerUnpublishVolume(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.ControllerUnpublishVolumeRequest
		expResponse *csi.ControllerUnpublishVolumeResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success detach volume",
			req:         &csi.ControllerUnpublishVolumeRequest{VolumeId: "volumeid", NodeId: "nodeid"},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.ControllerUnpublishVolume(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestValidateVolumeCapabilities(t *testing.T) {
	// test cases
	//confirmed := &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	testCases := []struct {
		name        string
		req         *csi.ValidateVolumeCapabilitiesRequest
		expResponse *csi.ValidateVolumeCapabilitiesResponse
		expErrCode  codes.Code
	}{
		//{
		//	name: "Success validate volume capabilities",
		//	req: &csi.ValidateVolumeCapabilitiesRequest{VolumeId: "volumeid",
		//		VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER}}},
		//	},
		//	expResponse: &csi.ValidateVolumeCapabilitiesResponse{Confirmed: confirmed},
		//	expErrCode:  codes.OK,
		//},
		{
			name: "Empty volume capabilities",
			req: &csi.ValidateVolumeCapabilitiesRequest{VolumeId: "volumeid",
				VolumeCapabilities: nil,
			},
			expResponse: nil,
			expErrCode:  codes.InvalidArgument,
		},
		{
			name: "Empty volume ID",
			req: &csi.ValidateVolumeCapabilitiesRequest{VolumeId: "",
				VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER}}},
			},
			expResponse: nil,
			expErrCode:  codes.InvalidArgument,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.ValidateVolumeCapabilities(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestCreateSnapshot(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.CreateSnapshotRequest
		expResponse *csi.CreateSnapshotResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success create snapshot",
			req:         &csi.CreateSnapshotRequest{},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.CreateSnapshot(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.DeleteSnapshotRequest
		expResponse *csi.DeleteSnapshotResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success delete snapshot",
			req:         &csi.DeleteSnapshotRequest{},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.DeleteSnapshot(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestListSnapshots(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.ListSnapshotsRequest
		expResponse *csi.ListSnapshotsResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success list snapshots",
			req:         &csi.ListSnapshotsRequest{},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.ListSnapshots(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestControllerExpandVolume(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.ControllerExpandVolumeRequest
		expResponse *csi.ControllerExpandVolumeResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success controller expand volume",
			req:         &csi.ControllerExpandVolumeRequest{},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.ControllerExpandVolume(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestListVolumes(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.ListVolumesRequest
		expResponse *csi.ListVolumesResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success list volumes",
			req:         &csi.ListVolumesRequest{},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.ListVolumes(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestGetCapacity(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.GetCapacityRequest
		expResponse *csi.GetCapacityResponse
		expErrCode  codes.Code
	}{
		{
			name:        "Success GetCapacity",
			req:         &csi.GetCapacityRequest{},
			expResponse: nil,
			expErrCode:  codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.GetCapacity(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

func TestControllerGetCapabilities(t *testing.T) {
	// test cases
	testCases := []struct {
		name        string
		req         *csi.ControllerGetCapabilitiesRequest
		expResponse *csi.ControllerGetCapabilitiesResponse
		expErrCode  codes.Code
	}{
		{
			name: "Success controller get capabilities",
			req:  &csi.ControllerGetCapabilitiesRequest{},
			expResponse: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{Type: &csi.ControllerServiceCapability_Rpc{Rpc: &csi.ControllerServiceCapability_RPC{Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME}}},
				},
			},
			expErrCode: codes.OK,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		// Call CSI CreateVolume
		response, err := icDriver.cs.ControllerGetCapabilities(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}

		if !reflect.DeepEqual(response, tc.expResponse) {
			assert.Equal(t, tc.expResponse, response)
		}
	}
}
