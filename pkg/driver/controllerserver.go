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
	PublishInfoRequestID = "request-id"
	maxStorageCapacity   = gib
	defaultIAMEndPoint   = "https://iam.bluemix.net"
)

type controllerServer struct {
	*csicommon.DefaultControllerServer
	*s3Driver
}

var getVolByName = getVolumeByName

func (cs *controllerServer) getCredentials(secretMap map[string]string) (*s3client.ObjectStorageCredentials, error) {
	var (
		accessKey         string
		secretKey         string
		apiKey            string
		serviceInstanceID string
		authType          string
	)

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
		ServiceInstanceID: serviceInstanceID,
	}, nil

}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var (
		bucketName string
		endPoint   string
		regnClass  string
		//objPath    string
	)
	klog.Infof("CSIControllerServer-CreateVolume... | Request: %v", *req)

	volumeName := sanitizeVolumeID(req.GetName())
	volumeID := volumeName
	caps := req.GetVolumeCapabilities()

	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Infof("Invalid create volume req: %v", *req)
		return nil, err
	}

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

	params := req.GetParameters()
	secretMap := req.GetSecrets()
	fmt.Println("CreateVolume Parameters:\n\t", params)
	fmt.Println("CreateVolume Secrets:\n\t", secretMap)

	creds, err := cs.getCredentials(req.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials: %v", err)
	}
	bucketName = secretMap["bucket-name"]

	if bucketName == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Bucket name is empty"))
	}

	endPoint = secretMap["cos-endpoint"]
	regnClass = secretMap["regn-class"]
	sess := s3client.NewObjectStorageSession(endPoint, regnClass, creds)

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

	params["cos-endpoint"] = endPoint
	params["regn-class"] = regnClass
	params["bucket-name"] = bucketName
	params["obj-path"] = secretMap["obj-path"]

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
	regnClass := secretMap["regn-class"]
	sess := s3client.NewObjectStorageSession(endPoint, regnClass, creds)
	sess.DeleteBucket(bucketName)

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
