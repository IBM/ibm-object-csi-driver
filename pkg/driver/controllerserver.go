/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"k8s.io/klog/v2"
	"strings"
)

const (
	// PublishInfoRequestID ...
	PublishInfoRequestID = "request-id"
	maxStorageCapacity   = gib
)

type controllerServer struct {
	*csicommon.DefaultControllerServer
	*s3Driver
}

var getVolByName = getVolumeByName

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.Infof("CSIControllerServer-ControllerPublishVolume | Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume")
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.Infof("CSIControllerServer-ControllerUnPublishVolume | Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume")
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var (
		val        string
		check      bool
		endPoint   string
		regnClass  string
		bucketName string
		objPath    string
		accessKey  string
		secretKey  string
		authType   string
	)
	klog.Infof("CSIControllerServer-CreateVolume... | Request: %v", *req)

	volumeName := sanitizeVolumeID(req.GetName())
	volumeID := volumeName

	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Infof("Invalid create volume req: %v", *req)
		return nil, err
	}

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	caps := req.GetVolumeCapabilities()
	if caps == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}
	for _, cap := range caps {
		if cap.GetBlock() != nil {
			return nil, status.Error(codes.InvalidArgument, "Block Volume not supported")
		}
	}
	// Check for already existing volume name
	if _, err := getVolByName(req.GetName()); err == nil {
		klog.Infof("Volume already exists %v", req.GetName())
		return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("Volume with the same name: %s exist", req.GetName()))
	}

	// Check for maximum available capacity
	capacity := int64(req.GetCapacityRange().GetRequiredBytes())
	if capacity >= maxStorageCapacity {
		return nil, status.Errorf(codes.OutOfRange, fmt.Sprintf("Requested capacity %d exceeds maximum allowed %d", capacity, maxStorageCapacity))
	}

	klog.Infof("Got a request to create volume: %s", volumeID)
	clinetCreds := &s3client.S3Credentials{}
	params := req.GetParameters()
	secretMap := req.GetSecrets()
	fmt.Println("CreateVolume Parameters:\n\t", params)
	fmt.Println("CreateVolume Secrets:\n\t", secretMap)

	if val, check := secretMap["cos-endpoint"]; check {
		endPoint = val
	}
	if val, check = secretMap["regn-class"]; check {
		regnClass = val
	}
	if val, check = secretMap["bucket-name"]; check {
		bucketName = val
	}
	if val, check = secretMap["obj-path"]; check {
		objPath = val
	}
	if val, check = secretMap["access-key"]; check {
		accessKey = val
		authType = "hmac"
	}
	if val, check = secretMap["secret-key"]; check {
		secretKey = val
	}

	//(cred *S3Credentials) SetCreds(authtype string, accesskey string, secretkey string, apikey string,  svcid string, iamep string) {
	clinetCreds.SetCreds(authType, accessKey, secretKey, "", "", "")

	//(client *s3Client) InitSession(endpoint string, class string, creds *s3Credentials)
	if err := cs.s3Driver.s3client.InitSession(endPoint, regnClass, clinetCreds); err != nil {
		klog.Error("CreateVolume Unable to initialize backend S3 Client")
		return nil, status.Error(codes.PermissionDenied, "Unable to initialize backend S3 Clinet")
	}

	if err := cs.s3Driver.s3client.CheckBucketAccess(bucketName); err != nil {
		klog.Error("CreateVolume Unable to access the bucket: %v", err)
		return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to to access the bucket: %v", bucketName))
	}

	params["cos-endpoint"] = endPoint
	params["regn-class"] = regnClass
	params["bucket-name"] = bucketName
	params["obj-path"] = objPath

	klog.Infof("create volume: %v", volumeID)

	s3Vol := s3Volume{}
	s3Vol.VolName = volumeName
	s3Vol.VolID = volumeID
	s3Vol.VolSize = capacity
	s3CosVolumes[volumeID] = s3Vol

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

	// Validate arguments
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Infof("Invalid delete volume req %v", *req)
		return nil, err
	}
	klog.Infof("Deleting volume %v", volumeID)
	klog.Infof("deleting volume %v", volumeID)
	//path := provisionRoot + volumeID
	//os.RemoveAll(path)
	delete(s3CosVolumes, volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.Infof("CSIControllerServer-ValidateVolumeCapabilities | Request: %v", *req)

	// Validate Arguments
	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if _, ok := s3CosVolumes[req.GetVolumeId()]; !ok {
		return nil, status.Error(codes.NotFound, "Volume does not exist")
	}
	return cs.DefaultControllerServer.ValidateVolumeCapabilities(ctx, req)

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
