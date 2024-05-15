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
	"fmt"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// Implements Controller csi.ControllerServer
type controllerServer struct {
	*S3Driver
	Stats      utils.StatsUtils
	cosSession s3client.ObjectStorageSessionFactory
	Logger     *zap.Logger
}

func (cs *controllerServer) CreateVolume(_ context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var (
		bucketName         string
		endPoint           string
		locationConstraint string
		kpRootKeyCrn       string
	)
	modifiedRequest, err := ReplaceAndReturnCopy(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in modifying requests %v", err))
	}
	klog.V(3).Infof("CSIControllerServer-CreateVolume: Request: %v", modifiedRequest.(*csi.CreateVolumeRequest))

	volumeName, err := sanitizeVolumeID(req.GetName())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in sanitizeVolumeID  %v", err))
	}
	volumeID := volumeName
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}

	caps := req.GetVolumeCapabilities()
	if caps == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}
	for _, cap := range caps {
		if cap.GetBlock() != nil {
			return nil, status.Error(codes.InvalidArgument, "Volume type block Volume not supported")
		}
	}

	// Check for maximum available capacity
	capacity := req.GetCapacityRange().GetRequiredBytes()
	if capacity >= constants.MaxStorageCapacity {
		return nil, status.Error(codes.OutOfRange, fmt.Sprintf("Requested capacity %d exceeds maximum allowed %d", capacity, constants.MaxStorageCapacity))
	}
	klog.Infof("Got a request to create volume: %s", volumeID)
	params := req.GetParameters()
	secretMap := req.GetSecrets()
	fmt.Println("CreateVolume Parameters:\n\t", params)
	creds, err := getCredentials(req.GetSecrets())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
	}
	endPoint = secretMap["cosEndpoint"]
	if endPoint == "" {
		return nil, status.Error(codes.InvalidArgument, "cosEndpoint unknown")
	}
	locationConstraint = secretMap["locationConstraint"]
	if locationConstraint == "" {
		return nil, status.Error(codes.InvalidArgument, "locationConstraint unknown")
	}

	kpRootKeyCrn = secretMap["kpRootKeyCRN"]
	if kpRootKeyCrn != "" {
		klog.Infof("key protect root key crn provided for bucket creation")
	}

	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)
	bucketName = secretMap["bucketName"]
	params["userProvidedBucket"] = "true"
	if bucketName != "" {
		// User Provided bucket. Check its existence and create if not present
		klog.Infof("Bucket name provided")
		if err := sess.CheckBucketAccess(bucketName); err != nil {
			klog.Infof("CreateVolume: Unable to access the bucket: %v, Creating with given name", err)
			err = createBucket(sess, bucketName, kpRootKeyCrn)
			if err != nil {
				return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("%v: %v", err, bucketName))
			}
			params["userProvidedBucket"] = "false"
			klog.Infof("Created bucket: %s", bucketName)
		}
		params["bucketName"] = bucketName
	} else {
		// Generate random temp bucket name based on volume id
		klog.Infof("Bucket name not provided")
		tempBucketName := getTempBucketName(secretMap["mounter"], volumeID)
		if tempBucketName == "" {
			klog.Errorf("CreateVolume: Unable to generate the bucket name")
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to access the bucket: %v", tempBucketName))
		}
		err = createBucket(sess, tempBucketName, kpRootKeyCrn)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("%v: %v", err, tempBucketName))
		}
		klog.Infof("Created temp bucket: %s", tempBucketName)
		params["userProvidedBucket"] = "false"
		params["bucketName"] = tempBucketName
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

func (cs *controllerServer) DeleteVolume(_ context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	modifiedRequest, err := ReplaceAndReturnCopy(req)
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

	creds, err := getCredentials(req.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials: %v", err)
	}

	endPoint := secretMap["cosEndpoint"]
	locationConstraint := secretMap["locationConstraint"]
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)

	bucketToDelete, err := cs.Stats.BucketToDelete(volumeID)
	if err != nil {
		return &csi.DeleteVolumeResponse{}, nil
	}

	if bucketToDelete != "" {
		err = sess.DeleteBucket(bucketToDelete)
		if err != nil {
			klog.V(3).Infof("Cannot delete temp bucket: %v; error msg: %v", bucketToDelete, err)
		}
		klog.Infof("End of bucket delete for  %v", volumeID)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ControllerPublishVolume(_ context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.V(3).Infof("CSIControllerServer-ControllerPublishVolume: Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume")
}

func (cs *controllerServer) ControllerUnpublishVolume(_ context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.V(3).Infof("CSIControllerServer-ControllerUnPublishVolume: Request: %v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume")
}

func (cs *controllerServer) ValidateVolumeCapabilities(_ context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(3).Infof("ValidateVolumeCapabilities: Request: %+v", *req)

	// Validate Arguments
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volCaps) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func (cs *controllerServer) ListVolumes(_ context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(3).Infof("ListVolumes: Request: %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ListVolumes")
}

func (cs *controllerServer) GetCapacity(_ context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(3).Infof("GetCapacity: Request: %+v", *req)
	return nil, status.Error(codes.Unimplemented, "GetCapacity")
}

func (cs *controllerServer) ControllerGetCapabilities(_ context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(3).Infof("ControllerGetCapabilities: Request: %+v", *req)
	var caps []*csi.ControllerServiceCapability
	for _, cap := range controllerCapabilities {
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

func (cs *controllerServer) CreateSnapshot(_ context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(3).Infof("CreateSnapshot: Request: %+v", *req)
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot")
}

func (cs *controllerServer) DeleteSnapshot(_ context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(3).Infof("DeleteSnapshot: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot")
}

func (cs *controllerServer) ListSnapshots(_ context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(3).Infof("ListSnapshots: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ListSnapshots")
}

func (cs *controllerServer) ControllerExpandVolume(_ context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(3).Infof("ControllerExpandVolume: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume")
}

func (cs *controllerServer) ControllerGetVolume(_ context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(3).Infof("ControllerGetVolume: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerGetVolume")
}

func (cs *controllerServer) ControllerModifyVolume(_ context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	klog.V(3).Infof("ControllerModifyVolume: called with args %+v", *req)
	return nil, status.Error(codes.Unimplemented, "ControllerModifyVolume")
}
