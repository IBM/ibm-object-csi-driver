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

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// Implements Controller csi.ControllerServer
type controllerServer struct {
	*S3Driver
	csi.UnimplementedControllerServer
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
		pvcName            string
		pvcNamespace       string
		bucketVersioning   string
	)
	secretMapCustom := make(map[string]string)

	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
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
	klog.Infof("Got a request to create volume: %s", volumeID)

	caps := req.GetVolumeCapabilities()
	if caps == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}
	for _, cap := range caps {
		klog.Infof("Volume capability: %s", cap)
		if cap.GetBlock() != nil {
			return nil, status.Error(codes.InvalidArgument, "Volume type block Volume not supported")
		}
	}

	params := req.GetParameters()
	klog.Info("CreateVolume Parameters:\n\t", params)

	secretMap := req.GetSecrets()
	klog.Info("req.GetSecrets() length:\t", len(secretMap))

	var customSecretName string
	if len(secretMap) == 0 {
		klog.Info("Did not find the secret that matches pvc name. Fetching custom secret from PVC annotations")

		pvcName = params[constants.PVCNameKey]
		pvcNamespace = params[constants.PVCNamespaceKey]

		if pvcName == "" {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("pvcName not specified, could not fetch the secret %v", err))
		}

		if pvcNamespace == "" {
			pvcNamespace = constants.DefaultNamespace
		}

		pvcRes, err := cs.Stats.GetPVC(pvcName, pvcNamespace)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("PVC resource not found %v", err))
		}

		klog.Info("pvc annotations:\n\t", pvcRes.Annotations)

		pvcAnnotations := pvcRes.Annotations

		customSecretName = pvcAnnotations[constants.SecretNameKey]
		secretNamespace := pvcAnnotations[constants.SecretNamespaceKey]

		if customSecretName == "" {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("secretName annotation 'cos.csi.driver/secret' not specified in the PVC annotations, could not fetch the secret %v", err))
		}

		if secretNamespace == "" {
			klog.Info("secretNamespace annotation 'cos.csi.driver/secret-namespace' not specified in PVC annotations:\t", pvcRes.Annotations, "\t trying to fetch the secret in default namespace")
			secretNamespace = constants.DefaultNamespace
		}

		secret, err := cs.Stats.GetSecret(customSecretName, secretNamespace)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Secret resource not found %v", err))
		}

		secretMapCustom = parseCustomSecret(secret)
		klog.Info("custom secret parameters parsed successfully, length of custom secret: ", len(secretMapCustom))

		secretMap = secretMapCustom
	}

	endPoint = secretMap["cosEndpoint"]
	if endPoint == "" {
		endPoint = params["cosEndpoint"]
	}
	if endPoint == "" {
		return nil, status.Error(codes.InvalidArgument, "cosEndpoint unknown")
	}

	locationConstraint = secretMap["locationConstraint"]
	if locationConstraint == "" {
		locationConstraint = params["locationConstraint"]
	}
	if locationConstraint == "" {
		return nil, status.Error(codes.InvalidArgument, "locationConstraint unknown")
	}

	kpRootKeyCrn = secretMap["kpRootKeyCRN"]
	if kpRootKeyCrn == "" {
		kpRootKeyCrn = secretMapCustom["kpRootKeyCRN"]
	}
	if kpRootKeyCrn != "" {
		klog.Infof("key protect root key crn provided for bucket creation")
	}

	mounter := secretMap["mounter"]
	if mounter == "" {
		mounter = params["mounter"]
	}

	bucketName = secretMap["bucketName"]
	if bucketName == "" {
		bucketName = secretMapCustom["bucketName"]
	}

	// Check for bucketVersioning parameter
	if val, ok := secretMap[constants.BucketVersioning]; ok && val != "" {
		enable := strings.ToLower(strings.TrimSpace(val))
		if enable != "true" && enable != "false" {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid BucketVersioning value in secret: %s. Value set %s. Must be 'true' or 'false'", customSecretName, val))
		}
		bucketVersioning = enable
		klog.Infof("BucketVersioning value that will be set via secret: %s", bucketVersioning)
	} else if val, ok := params[constants.BucketVersioning]; ok && val != "" {
		enable := strings.ToLower(strings.TrimSpace(val))
		if enable != "true" && enable != "false" {
			return nil, status.Error(codes.InvalidArgument,
				fmt.Sprintf("Invalid bucketVersioning value in storage class: %s. Must be 'true' or 'false'", val))
		}
		bucketVersioning = enable
		klog.Infof("BucketVersioning value that will be set via storage class params: %s", bucketVersioning)
	}

	creds, err := getCredentials(secretMap)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
	}

	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)

	params["userProvidedBucket"] = "true"
	if bucketName != "" {
		// User Provided bucket. Check its existence and create if not present
		klog.Infof("Bucket name provided: %v", bucketName)
		klog.Infof("Check if the provided bucket already exists: %v", bucketName)
		if err := sess.CheckBucketAccess(bucketName); err != nil {
			klog.Infof("CreateVolume: bucket not accessible: %v, Creating new bucket with given name", err)
			err = createBucket(sess, bucketName, kpRootKeyCrn)
			if err != nil {
				return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("%v: %v", err, bucketName))
			}
			params["userProvidedBucket"] = "false"
			klog.Infof("Created bucket: %s", bucketName)
		}

		if bucketVersioning != "" {
			enable := strings.ToLower(strings.TrimSpace(bucketVersioning)) == "true"
			klog.Infof("Bucket versioning value evaluated to: %t", enable)

			err := sess.SetBucketVersioning(bucketName, enable)
			if err != nil {
				if params["userProvidedBucket"] == "false" {
					err1 := sess.DeleteBucket(bucketName)
					if err1 != nil {
						return nil, status.Error(codes.Internal, fmt.Sprintf("cannot set versioning: %v and cannot delete bucket %s: %v", err, bucketName, err1))
					}
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("failed to set versioning %t for bucket %s: %v", enable, bucketName, err))
			}
			klog.Infof("Bucket versioning set to %t for bucket %s", enable, bucketName)
		}

		params["bucketName"] = bucketName
	} else {
		// Generate random temp bucket name based on volume id
		klog.Infof("Bucket name not provided")
		tempBucketName := getTempBucketName(mounter, volumeID)
		if tempBucketName == "" {
			klog.Errorf("CreateVolume: Unable to generate the bucket name")
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to access the bucket: %v", tempBucketName))
		}
		err = createBucket(sess, tempBucketName, kpRootKeyCrn)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("%v: %v", err, tempBucketName))
		}

		if bucketVersioning != "" {
			enable := strings.ToLower(strings.TrimSpace(bucketVersioning)) == "true"
			klog.Infof("Temp bucket versioning value evaluated to: %t", enable)

			err := sess.SetBucketVersioning(tempBucketName, enable)
			if err != nil {
				err1 := sess.DeleteBucket(tempBucketName)
				if err1 != nil {
					return nil, status.Error(codes.Internal, fmt.Sprintf("cannot set versioning: %v and cannot delete temp bucket %s: %v", err, tempBucketName, err1))
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("failed to set versioning %t for temp bucket %s: %v", enable, tempBucketName, err))
			}
			klog.Infof("Bucket versioning set to %t for temp bucket %s", enable, tempBucketName)
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
	//secretMapCustom := make(map[string]string)

	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
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

	endPoint := secretMap["cosEndpoint"]
	locationConstraint := secretMap["locationConstraint"]

	if len(secretMap) == 0 {
		klog.Info("Did not find the secret that matches pvc name. Fetching custom secret from PVC annotations")

		pv, err := cs.Stats.GetPV(volumeID)
		if err != nil {
			return nil, err
		}

		klog.Info("pv Resource details:\n\t", pv)

		secretName := pv.Spec.CSI.NodePublishSecretRef.Name
		secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace

		if secretName == "" {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Secret details not found, could not fetch the secret %v", err))
		}

		if secretNamespace == "" {
			klog.Info("secret Namespace not found. trying to fetch the secret in default namespace")
			secretNamespace = constants.DefaultNamespace
		}

		endPoint = pv.Spec.CSI.VolumeAttributes["cosEndpoint"]
		locationConstraint = pv.Spec.CSI.VolumeAttributes["locationConstraint"]

		klog.Info("secret details found. secret-name: ", secretName, "\tsecret-namespace: ", secretNamespace)

		secret, err := cs.Stats.GetSecret(secretName, secretNamespace)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Secret resource not found %v", err))
		}

		secretMapCustom := parseCustomSecret(secret)
		klog.Info("custom secret parameters parsed successfully, length of custom secret: ", len(secretMapCustom))
		secretMap = secretMapCustom
	}

	creds, err := getCredentials(secretMap)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
	}

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
	klog.V(3).Infof("CSIControllerServer-ControllerPublishVolume: Request: %+v", req)
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume")
}

func (cs *controllerServer) ControllerUnpublishVolume(_ context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.V(3).Infof("CSIControllerServer-ControllerUnPublishVolume: Request: %+v", req)
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume")
}

func (cs *controllerServer) ValidateVolumeCapabilities(_ context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(3).Infof("ValidateVolumeCapabilities: Request: %+v", req)

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
	klog.V(3).Infof("ListVolumes: Request: %+v", req)
	return nil, status.Error(codes.Unimplemented, "ListVolumes")
}

func (cs *controllerServer) GetCapacity(_ context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(3).Infof("GetCapacity: Request: %+v", req)
	return nil, status.Error(codes.Unimplemented, "GetCapacity")
}

func (cs *controllerServer) ControllerGetCapabilities(_ context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(3).Infof("ControllerGetCapabilities: Request: %+v", req)
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
	klog.V(3).Infof("CreateSnapshot: Request: %+v", req)
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot")
}

func (cs *controllerServer) DeleteSnapshot(_ context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(3).Infof("DeleteSnapshot: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot")
}

func (cs *controllerServer) ListSnapshots(_ context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(3).Infof("ListSnapshots: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "ListSnapshots")
}

func (cs *controllerServer) ControllerExpandVolume(_ context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(3).Infof("ControllerExpandVolume: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume")
}

func (cs *controllerServer) ControllerGetVolume(_ context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(3).Infof("ControllerGetVolume: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "ControllerGetVolume")
}

func (cs *controllerServer) ControllerModifyVolume(_ context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	klog.V(3).Infof("ControllerModifyVolume: called with args %+v", req)
	return nil, status.Error(codes.Unimplemented, "ControllerModifyVolume")
}

func getCredentials(secretMap map[string]string) (*s3client.ObjectStorageCredentials, error) {
	klog.Infof("- getCredentials-")
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
		iamEndpoint = constants.DefaultIAMEndPoint
	}

	if val, check := secretMap["apiKey"]; check {
		apiKey = val
	}

	// Add In Docs APIKEY is require param in secret
	authType = "iam"
	serviceInstanceID = secretMap["serviceId"]
	if serviceInstanceID == "" {
		accessKey = secretMap["accessKey"]
		secretKey = secretMap["secretKey"]
		if accessKey == "" || secretKey == "" {
			return nil, status.Error(codes.Unauthenticated, "Valid access credentials are not provided in the secret| serviceId/accessKey/secretKey unknown")
		}
		authType = "hmac"
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

func parseCustomSecret(secret *v1.Secret) map[string]string {
	klog.Infof("-parseCustomSecret-")
	secretMapCustom := make(map[string]string)

	var (
		accessKey          string
		secretKey          string
		apiKey             string
		serviceInstanceID  string
		kpRootKeyCrn       string
		bucketName         string
		iamEndpoint        string
		cosEndpoint        string
		locationConstraint string
		bucketVersioning   string
	)

	if bytesVal, ok := secret.Data["accessKey"]; ok {
		accessKey = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["secretKey"]; ok {
		secretKey = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["apiKey"]; ok {
		apiKey = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["kpRootKeyCRN"]; ok {
		kpRootKeyCrn = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["serviceId"]; ok {
		serviceInstanceID = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["bucketName"]; ok {
		bucketName = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["iamEndpoint"]; ok {
		iamEndpoint = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["cosEndpoint"]; ok {
		cosEndpoint = string(bytesVal)
	}

	if bytesVal, ok := secret.Data["locationConstraint"]; ok {
		locationConstraint = string(bytesVal)
	}

	if bytesVal, ok := secret.Data[constants.BucketVersioning]; ok {
		bucketVersioning = string(bytesVal)
	}

	secretMapCustom["accessKey"] = accessKey
	secretMapCustom["secretKey"] = secretKey
	secretMapCustom["apiKey"] = apiKey
	secretMapCustom["kpRootKeyCRN"] = kpRootKeyCrn
	secretMapCustom["serviceId"] = serviceInstanceID
	secretMapCustom["bucketName"] = bucketName
	secretMapCustom["iamEndpoint"] = iamEndpoint
	secretMapCustom["cosEndpoint"] = cosEndpoint
	secretMapCustom["locationConstraint"] = locationConstraint
	secretMapCustom[constants.BucketVersioning] = bucketVersioning

	return secretMapCustom
}

func getTempBucketName(mounterType, volumeID string) string {
	klog.Infof("mounterType: %v", mounterType)
	currentTime := time.Now()
	timestamp := currentTime.Format("20060102150405")

	name := fmt.Sprintf("%s%s-%s", mounterType, timestamp, volumeID)
	return name
}

func createBucket(sess s3client.ObjectStorageSession, bucketName, kpRootKeyCrn string) error {
	msg, err := sess.CreateBucket(bucketName, kpRootKeyCrn)
	if msg != "" {
		klog.Infof("Info:Create Volume module with user provided Bucket name: %v", msg)
	}
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "BucketAlreadyExists" {
			klog.Warning(fmt.Sprintf("bucket '%s' already exists", bucketName))
		} else {
			klog.Errorf("CreateVolume: Unable to create the bucket: %v", err)
			return errors.New("unable to create the bucket")
		}
	}
	if err := sess.CheckBucketAccess(bucketName); err != nil {
		klog.Errorf("CreateVolume: Unable to access the bucket: %v", err)
		return errors.New("unable to access the bucket")
	}
	return nil
}

func sanitizeVolumeID(volumeID string) (string, error) {
	var err error
	volumeID = strings.ToLower(volumeID)
	if len(volumeID) > 63 {
		h := sha256.New()
		_, err = io.WriteString(h, volumeID)
		volumeID = hex.EncodeToString(h.Sum(nil))
	}
	return volumeID, err
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(capacity *csi.VolumeCapability) bool {
		for _, c := range volumeCapabilities {
			volumeCap := csi.VolumeCapability_AccessMode{
				Mode: c,
			}
			if volumeCap.GetMode() == capacity.AccessMode.GetMode() {
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
