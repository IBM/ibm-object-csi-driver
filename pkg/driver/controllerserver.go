/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	PublishInfoRequestID = "request-id"
	maxStorageCapacity   = gib
	defaultIAMEndPoint   = "https://iam.cloud.ibm.com"
)

// Implements Controller csi.ControllerServer
type controllerServer struct {
	*S3Driver
	cosSession s3client.ObjectStorageSessionFactory
	Logger     *zap.Logger
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

	if val, check := secretMap["iamEndpoint"]; check {
		iamEndpoint = val
	}
	if iamEndpoint == "" {
		iamEndpoint = defaultIAMEndPoint
	}

	if val, check := secretMap["apiKey"]; check {
		apiKey = val
	}

	if apiKey == "" {
		authType = "hmac"
		accessKey = secretMap["accessKey"]
		if accessKey == "" {
			return nil, status.Error(codes.Unauthenticated, "Valid access credentials are not provided in the secret| accessKey unknown")
		}

		secretKey = secretMap["secretKey"]
		if secretKey == "" {
			return nil, status.Error(codes.Unauthenticated, "Valid access credentials are not provided in the secret| secretKey unknown")
		}
	} else {
		authType = "iam"
		serviceInstanceID = secretMap["serviceId"]
		if serviceInstanceID == "" {
			return nil, status.Error(codes.Unauthenticated, "Valid access credentials are not provided in the secret| serviceId  unknown")
		}
	}

	return &s3client.ObjectStorageCredentials{
		AuthType:          authType,
		AccessKey:         accessKey,
		SecretKey:         secretKey,
		APIKey:            apiKey,
		IAMEndpoint:       iamEndpoint,
		ServiceInstanceID: serviceInstanceID,
	}, nil

}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var (
		bucketName         string
		endPoint           string
		locationConstraint string
		//objPath    string
	)
	modifiedRequest, err := ReplaceAndReturnCopy(req, "xxx", "yyy")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in modifying requests %v", err))
	}
	klog.V(3).Infof("CSIControllerServer-CreateVolume: Request: %v", modifiedRequest.(*csi.CreateVolumeRequest))

	volumeName, err := sanitizeVolumeID(req.GetName())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in sanitizeVolumeID  %v", err))
	}
	volumeID := volumeName
	caps := req.GetVolumeCapabilities()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if caps == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}
	for _, cap := range caps {
		if cap.GetBlock() != nil {
			return nil, status.Error(codes.InvalidArgument, "Volume type block Volume not supported")
		}
	}

	// Check for maximum available capacity
	capacity := int64(req.GetCapacityRange().GetRequiredBytes())
	if capacity >= maxStorageCapacity {
		return nil, status.Error(codes.OutOfRange, fmt.Sprintf("Requested capacity %d exceeds maximum allowed %d", capacity, maxStorageCapacity))
	}

	params := req.GetParameters()
	secretMap := req.GetSecrets()
	fmt.Println("CreateVolume Parameters:\n\t", params)

	creds, err := cs.getCredentials(req.GetSecrets())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
	}
	bucketName = secretMap["bucketName"]

	if bucketName == "" {
		return nil, status.Error(codes.InvalidArgument, "bucketName unknown")
	}
	klog.Infof("-CreateVolume-: volumeID: %s   bucketName: %s", volumeID, bucketName)

	endPoint = secretMap["cosEndpoint"]
	if endPoint == "" {
		return nil, status.Error(codes.InvalidArgument, "cosEndpoint unknown")
	}
	locationConstraint = secretMap["locationConstraint"]
	if locationConstraint == "" {
		return nil, status.Error(codes.InvalidArgument, "locationConstraint unknown")
	}
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)

	msg, err := sess.CreateBucket(bucketName)
	if msg != "" {
		klog.Infof("-CreateVolume-: Create bucket:", msg)
	}
	if err != nil {

		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "BucketAlreadyExists" {
			klog.Warning(fmt.Sprintf("bucket '%s' already exists", bucketName))
		} else {
			klog.Errorf("CreateVolume: Unable to create the bucket: %v", err)
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to create the bucket: %v", bucketName))
		}

	}

	if err := sess.CheckBucketAccess(bucketName); err != nil {
		klog.Errorf("CreateVolume: Unable to access the bucket: %v", err)
		return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to access the bucket: %v", bucketName))
	}

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
	modifiedRequest, err := ReplaceAndReturnCopy(req, "xxx", "yyy")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in modifying requests %v", err))
	}
	klog.V(3).Infof("CSIControllerServer-DeleteVolume: Request: %v", modifiedRequest.(*csi.DeleteVolumeRequest))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	klog.Infof("Deleting volume %v", volumeID)
	secretMap := req.GetSecrets()

	creds, err := cs.getCredentials(req.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials: %v", err)
	}
	bucketName := secretMap["bucketName"]

	if bucketName == "" {
		return nil, status.Error(codes.InvalidArgument, "Bucket name is empty")
	}

	endPoint := secretMap["cosEndpoint"]
	locationConstraint := secretMap["locationConstraint"]
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)
	err = sess.DeleteBucket(bucketName)
	if err != nil {
		return nil, fmt.Errorf("cannot delete bucket: %v", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume
func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.V(3).Infof("CSIControllerServer-ControllerPublishVolume: Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume")
}

// ControllerUnpublishVolume
func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.V(3).Infof("CSIControllerServer-ControllerUnPublishVolume: Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume")
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(3).Infof("ValidateVolumeCapabilities: Request: %+v", *req)
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

// ListVolumes
func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(3).Infof("ListVolumes: Request: %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ListVolumes")
}

// GetCapacity ...
func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(3).Infof("GetCapacity: Request: %+v", *req)
	return nil, status.Error(codes.Unimplemented, "GetCapacity")
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(3).Infof("ControllerGetCapabilities: Request: %+v", *req)
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

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(3).Infof("CreateSnapshot: Request: %+v", *req)
	return nil, status.Error(codes.Unimplemented, "")

}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(3).Infof("DeleteSnapshot: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot")
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(3).Infof("ListSnapshots: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ListSnapshots")
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(3).Infof("ControllerExpandVolume: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume")
}

func (cs *controllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(3).Infof("ControllerGetVolume: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "")
}

func sanitizeVolumeID(volumeID string) (string, error) {
	var err error
	volumeID = strings.ToLower(volumeID)
	if len(volumeID) > 63 {
		h := sha256.New()
		_, err = io.WriteString(h, volumeID) //nolint
		volumeID = hex.EncodeToString(h.Sum(nil))
	}
	return volumeID, err
}
