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
package driver

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"k8s.io/klog/v2"
	"strings"
)

const (
	PublishInfoRequestID = "request-id"
	maxStorageCapacity   = gib
	defaultIAMEndPoint   = "https://iam.cloud.ibm.com"
)

type controllerServer struct {
	*s3Driver
	newSession s3client.ObjectStorageSessionFactory
}

var (
	// volumeCaps represents how the volume could be accessed.
	volumeCaps = []csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		},
	}

	// controllerCaps represents the capability of controller service
	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	}
)

func (cs *controllerServer) getCredentials(secretMap map[string]string) (*s3client.ObjectStorageCredentials, error) {
	var (
		accessKey         string
		secretKey         string
		apiKey            string
		serviceInstanceID string
		authType          string
		iamEndpoint       string
	)

	iamEndpoint =  secretMap["iam-endpoint"]
	if iamEndpoint == "" {
		iamEndpoint = defaultIAMEndPoint
	}
	apiKey = secretMap["api-key"]
	if apiKey == "" {
		authType = "hmac"
		accessKey = secretMap["access-key"]
		if accessKey == "" {
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("Valid access credentials are not provided in the secret| access-key missing"))
		}

		secretKey = secretMap["secret-key"]
		if secretKey == "" {
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("Valid access credentials are not provided in the secret| secret-key missing"))
		}
	} else {
		authType = "iam"
		serviceInstanceID = secretMap["service-id"]
		if serviceInstanceID == "" {
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("Valid access credentials are not provided in the secret| serviceInstanceID  missing"))
		}
	}

	return &s3client.ObjectStorageCredentials{
		AuthType:          authType,
		AccessKey:         accessKey,
		SecretKey:         secretKey,
		APIKey:            apiKey,
		IAMEndpoint      : iamEndpoint,
	}, nil

}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var (
		bucketName string
		endPoint   string
		locationConstraint  string
		//objPath    string
	)
	klog.Infof("CSIControllerServer-CreateVolume... | Request: %v", *req)

	volumeName := sanitizeVolumeID(req.GetName())
	volumeID := volumeName
	caps := req.GetVolumeCapabilities()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	if caps == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}
	for _, cap := range caps {
		if cap.GetBlock() != nil {
			return nil, status.Error(codes.InvalidArgument, "Block Volume not supported")
		}
	}

	// Check for maximum available capacity
	capacity := int64(req.GetCapacityRange().GetRequiredBytes())
	if capacity >= maxStorageCapacity {
		return nil, status.Error(codes.OutOfRange, fmt.Sprintf("Requested capacity %d exceeds maximum allowed %d", capacity, maxStorageCapacity))
	}

	klog.Infof("Got a request to create volume: %s", volumeID)

	params := req.GetParameters()
	secretMap := req.GetSecrets()
	fmt.Println("CreateVolume Parameters:\n\t", params)
	fmt.Println("CreateVolume Secrets:\n\t", secretMap)

	creds, err := cs.getCredentials(req.GetSecrets())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
	}
	bucketName = secretMap["bucket-name"]

	if bucketName == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Bucket name is empty"))
	}
	endPoint = secretMap["cos-endpoint"]
	if endPoint == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("No endpoint value provided"))
	}
	locationConstraint = secretMap["location-constraint"]
	if locationConstraint == "" {
                return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("No locationConstraint value provided"))
        }
	sess := cs.newSession.NewObjectStorageSession(endPoint, locationConstraint, creds)

	msg, err := sess.CreateBucket(bucketName)
	if msg != "" {
		klog.Infof("Info:Create Volume module:", msg)
	}
	if err != nil {
		klog.Error("CreateVolume: Unable to create the bucket: %v", err)
		return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to create the bucket: %v", bucketName))
	}

	if err := sess.CheckBucketAccess(bucketName); err != nil {
		klog.Error("CreateVolume: Unable to access the bucket: %v", err)
		return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to access the bucket: %v", bucketName))
	}
	/*params["cos-endpoint"] = endPoint
	params["location-constraint"] = locationConstraint
	params["bucket-name"] = bucketName
	params["obj-path"] = secretMap["obj-path"]*/

	klog.Infof("create volume: %v", volumeID)

	//COS Endpoint, bucket, access keys will be stored in the csiProvisionerSecretName
	//The other tunables will be SC Parameters like ibm.io/multireq-max and other
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeID,
			CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
			VolumeContext: params,
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.Infof("CSIControllerServer-DeleteVolume... %v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	klog.Infof("Deleting volume %v", volumeID)
	klog.Infof("deleting volume %v", volumeID)
	secretMap := req.GetSecrets()
	fmt.Println("DeleteVolume Secrets:\n\t", secretMap)

	creds, err := cs.getCredentials(req.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials: %v", err)
	}
	bucketName := secretMap["bucket-name"]

	if bucketName == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Bucket name is empty"))
	}

	endPoint := secretMap["cos-endpoint"]
	locationConstraint := secretMap["location-constraint"]
	sess := cs.newSession.NewObjectStorageSession(endPoint, locationConstraint, creds)
	sess.DeleteBucket(bucketName)

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: called with args %+v", *req)
	// Validate Arguments

	volumeID := req.GetVolumeId()
	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volCaps) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil

}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range volumeCaps {
			if c.GetMode() == cap.AccessMode.GetMode() {
				return true
			}
		}
		return false
	}

	foundAll := true
	for _, c := range volCaps {
		if !hasSupport(c) {
			foundAll = false
		}
	}
	return foundAll
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")

}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {

	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot")
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {

	return nil, status.Error(codes.Unimplemented, "ListSnapshots")
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume")
}

func sanitizeVolumeID(volumeID string) string {
	volumeID = strings.ToLower(volumeID)
	if len(volumeID) > 63 {
		h := sha1.New()
		io.WriteString(h, volumeID)
		volumeID = hex.EncodeToString(h.Sum(nil))
	}
	return volumeID
}

// GetCapacity ...
func (csiCS *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {

	return nil, status.Error(codes.Unimplemented, "GetCapacity")
}

//ListVolumes
func (csiCS *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {

	return nil, status.Error(codes.Unimplemented, "ListVolumes")
}

//ControllerPublishVolume
func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.Infof("CSIControllerServer-ControllerPublishVolume | Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume")
}

//ControllerUnpublishVolume
func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.Infof("CSIControllerServer-ControllerUnPublishVolume | Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume")
}

func (d *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Infof("ControllerGetCapabilities: called with args %+v", *req)
	var caps []*csi.ControllerServiceCapability
	for _, cap := range controllerCaps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}
