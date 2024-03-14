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
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	volumeCaps = &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
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
	klog.Infof("Got a request to create volume: %s", volumeID)
	params := req.GetParameters()
	secretMap := req.GetSecrets()
	fmt.Println("CreateVolume Parameters:\n\t", params)
	creds, err := cs.getCredentials(req.GetSecrets())
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
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)
	bucketName = secretMap["bucketName"]
	params["userProvidedBucket"] = "true"
	if bucketName != "" {
		// User Provided bucket. Check its existence and create if not present
		klog.Infof("Bucket name provided")
		if err := sess.CheckBucketAccess(bucketName); err != nil {
			klog.Infof("CreateVolume: Unable to access the bucket: %v, Creating with given name", err)
			err = createBucket(sess, bucketName)
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
		err = createBucket(sess, tempBucketName)
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

	endPoint := secretMap["cosEndpoint"]
	locationConstraint := secretMap["locationConstraint"]
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)
	bucketToDelete, err := bucketToDelete(volumeID)

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

	for _, capability := range req.VolumeCapabilities {
		if capability.GetAccessMode().GetMode() != volumeCaps.GetMode() {
			return &csi.ValidateVolumeCapabilitiesResponse{Message: "Only multi node multi writer is supported"}, nil
		}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: volumeCaps,
				},
			},
		},
	}, nil


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

func getTempBucketName(mounterType, volumeID string) string {
	currentTime := time.Now()
	timestamp := currentTime.Format("20060102150405")

	name := fmt.Sprintf("%s%s-%s", mounterType, timestamp, volumeID)
	return name
}

func bucketToDelete(volumeID string) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("Unable to fetch bucket %v", err)
		return "", err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("Unable to fetch bucket %v", err)
		return "", err
	}

	pv, err := clientset.CoreV1().PersistentVolumes().Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Unable to fetch bucket %v", err)
		return "", err
	}

	klog.Infof("***Attributes", pv.Spec.CSI.VolumeAttributes)
	if string(pv.Spec.CSI.VolumeAttributes["userProvidedBucket"]) != "true" {

		klog.Infof("Bucket will be deleted %v", pv.Spec.CSI.VolumeAttributes["bucketName"])
		return pv.Spec.CSI.VolumeAttributes["bucketName"], nil
	}
	klog.Infof("Bucket will be persisted %v", pv.Spec.CSI.VolumeAttributes["bucketName"])
	return "", nil

}

func createBucket(sess s3client.ObjectStorageSession, bucketName string) error {
	msg, err := sess.CreateBucket(bucketName)
	if msg != "" {
		klog.Infof("Info:Create Volume module with user provided Bucket name:", msg)
	}
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "BucketAlreadyExists" {
			klog.Warning(fmt.Sprintf("bucket '%s' already exists", bucketName))
		} else {
			klog.Errorf("CreateVolume: Unable to create the bucket: %v", err)
			return errors.New("Unable to create the bucket")
		}
	}
	if err := sess.CheckBucketAccess(bucketName); err != nil {
		klog.Errorf("CreateVolume: Unable to access the bucket: %v", err)
		return errors.New("Unable to access the bucket")
	}
	return nil

}
