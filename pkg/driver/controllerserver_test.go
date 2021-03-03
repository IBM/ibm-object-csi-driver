/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

//Package driver ...
package driver

import (
	providerError "github.com/IBM/ibmcloud-storage-volume-lib/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

var (
	// Define "normal" parameters
	stdVolCap = []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{FsType: "ext2"},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	stdVolCapNotSupported = []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{FsType: "ext2"},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			},
		},
	}
	stdBlockVolCap = []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Block{
				Block: &csi.VolumeCapability_BlockVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	stdCapRange = &csi.CapacityRange{
		RequiredBytes: 20 * 1024 * 1024,
	}
	stdCapOutOfRange = &csi.CapacityRange{
		RequiredBytes: 20 * 1024 * 1024 * 1024,
	}
)

func TestCreateVolumeArguments(t *testing.T) {
	cap := 20
	volName := "test-name"
	iopsStr := ""
	// test cases
	testCases := []struct {
		name              string
		req               *csi.CreateVolumeRequest
		expVol            *csi.Volume
		expErrCode        codes.Code
		libVolumeResponse *provider.Volume
		libVolumeError    error
	}{
		{
			name: "Success default",
			req: &csi.CreateVolumeRequest{
				Name:               volName,
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
			},
			expVol: &csi.Volume{
				CapacityBytes: 20 * 1024 * 1024, // In byte
				VolumeId:      "testVolumeId",
				VolumeContext: map[string]string{utils.NodeRegionLabel: "myregion", utils.NodeZoneLabel: "myzone"},
			},
			libVolumeResponse: &provider.Volume{Capacity: &cap, Name: &volName, VolumeID: "testVolumeId", Iops: &iopsStr, Az: "myzone", Region: "myregion"},
			expErrCode:        codes.PermissionDenied,
			libVolumeError:    nil,
		},
		{
			name: "Empty volume name",
			req: &csi.CreateVolumeRequest{
				Name:               "",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
			},
			expVol:            nil,
			libVolumeResponse: nil,
			expErrCode:        codes.InvalidArgument,
			libVolumeError:    nil,
		},
		{
			name: "Empty volume capabilities",
			req: &csi.CreateVolumeRequest{
				Name:               volName,
				CapacityRange:      stdCapRange,
				VolumeCapabilities: nil,
			},
			expVol:            nil,
			libVolumeResponse: nil,
			expErrCode:        codes.InvalidArgument,
			libVolumeError:    nil,
		},
		{
			name: "Not supported volume Capabilities",
			req: &csi.CreateVolumeRequest{
				Name:               volName,
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdBlockVolCap,
			},
			expVol:            nil,
			libVolumeResponse: nil,
			expErrCode:        codes.InvalidArgument,
			libVolumeError:    nil,
		},
		{
			name: "Requested capacity out of Range",
			req: &csi.CreateVolumeRequest{
				Name:               volName,
				CapacityRange:      stdCapOutOfRange,
				VolumeCapabilities: stdVolCap,
			},
			expVol:            nil,
			libVolumeResponse: nil,
			expErrCode:        codes.OutOfRange,
			libVolumeError:    nil,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		// Call CSI CreateVolume
		resp, err := icDriver.cs.CreateVolume(context.Background(), tc.req)
		if err != nil {
			//errorType := providerError.GetErrorType(err)
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", serverError)
			}
			if serverError.Code() != tc.expErrCode {
				t.Fatalf("Expected error code-> %v, Actual error code: %v. err : %v", tc.expErrCode, serverError.Code(), err)
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error-> %v, actual no error", tc.expErrCode)
		}

		// Make sure responses match
		vol := resp.GetVolume()
		if vol == nil {
			t.Fatalf("Expected volume-> %v, Actual volume is nil", tc.expVol)
		}

	}
}

func TestDeleteVolume(t *testing.T) {
	// test cases
	testCases := []struct {
		name                 string
		req                  *csi.DeleteVolumeRequest
		expResponse          *csi.DeleteVolumeResponse
		expErrCode           codes.Code
		libVolumeResponse    error
		libVolumeGetResponce *provider.Volume
	}{
		{
			name:                 "Success volume delete",
			req:                  &csi.DeleteVolumeRequest{VolumeId: "testVolumeId"},
			expResponse:          &csi.DeleteVolumeResponse{},
			expErrCode:           codes.OK,
			libVolumeResponse:    nil,
			libVolumeGetResponce: &provider.Volume{VolumeID: "testVolumeId", Az: "myzone", Region: "myregion"},
		},
		{
			name:                 "Failed volume delete with volume id empty",
			req:                  &csi.DeleteVolumeRequest{VolumeId: ""},
			expResponse:          nil,
			expErrCode:           codes.InvalidArgument,
			libVolumeResponse:    nil,
			libVolumeGetResponce: nil,
		},
		{
			name:                 "Failed lib volume delete failed",
			req:                  &csi.DeleteVolumeRequest{VolumeId: ""},
			expResponse:          nil,
			expErrCode:           codes.Internal,
			libVolumeResponse:    providerError.Message{Code: "FailedToDeleteVolume", Description: "Volume deletion failed", Type: providerError.DeletionFailed},
			libVolumeGetResponce: &provider.Volume{VolumeID: "testVolumeId", Az: "myzone", Region: "myregion"},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Logf("test case: %s", tc.name)
		// Setup new driver each time so no interference
		icDriver := inits3Driver(t)

		response, err := icDriver.cs.DeleteVolume(context.Background(), tc.req)
		if tc.expErrCode != codes.OK {
			t.Logf("Error code")
			assert.NotNil(t, err)
		}
		assert.Equal(t, tc.expResponse, response)
	}
}

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
	testCases := []struct {
		name        string
		req         *csi.ValidateVolumeCapabilitiesRequest
		expResponse *csi.ValidateVolumeCapabilitiesResponse
		expErrCode  codes.Code
	}{
		{
			name: "Success validate volume capabilities",
			req: &csi.ValidateVolumeCapabilitiesRequest{VolumeId: "volumeid",
				VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}}},
			},
			expResponse: nil,
			expErrCode:  codes.Unimplemented,
		},
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
				VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}}},
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
