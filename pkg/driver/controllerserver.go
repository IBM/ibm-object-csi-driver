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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Implements Controller csi.ControllerServer
type controllerServer struct {
	*S3Driver
	Stats      utils.StatsUtils
	cosSession s3client.ObjectStorageSessionFactory
	Logger     *zap.Logger
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var (
		bucketName         string
		endPoint           string
		locationConstraint string
		kpRootKeyCrn       string
	)
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
		if cap.GetBlock() != nil {
			return nil, status.Error(codes.InvalidArgument, "Volume type block Volume not supported")
		}
	}

	params := req.GetParameters()
	klog.Infof("CreateVolume Parameters:\n\t", params)

	secretMap := req.GetSecrets()
	klog.Infof("Secret Parameters:\n\t", secretMap)
	creds, err := getCredentials(req.GetSecrets())
	if err != nil {
		klog.Info("Got error with getCredentials, trying to pull custom secret\n\t")
		// add logic to parse secret from secretname
		client, err := createK8sClient()
		if err != nil {
			klog.Error("Error creating Kubernetes clientset: ", err)
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
		}

		secretName := "custom-secret"
		secretNamespace := "default"

		accessKey, secretKey, apiKey, kpRootKeyCrn, err := getCredentialsCustom(ctx, secretName, secretNamespace, client)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
		}
		klog.Info("Custom secret Parameters:\n\t", accessKey, secretKey, apiKey, kpRootKeyCrn)
		//frame secretmap with all the above values and pass to getCrdentials as it is used to initialise cos session
		secretMap = make(map[string]string)
		secretMap["accessKey"] = accessKey
		secretMap["secretKey"] = secretKey
		secretMap["apiKey"] = apiKey
		secretMap["kpRootKeyCrn"] = kpRootKeyCrn

		creds, err = getCredentials(secretMap)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in getting credentials %v", err))
		}
	}

	endPoint = secretMap["cosEndpoint"]
	locationConstraint = secretMap["locationConstraint"]

	if endPoint == "" {
		endPoint = params["cosEndpoint"]
	}
	if locationConstraint == "" {
		locationConstraint = params["locationConstraint"]
	}

	if endPoint == "" {
		return nil, status.Error(codes.InvalidArgument, "cosEndpoint unknown")
	}
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

func createK8sClient() (*kubernetes.Clientset, error) {
	// Create a Kubernetes client configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Error("Error creating Kubernetes client configuration: ", err)
		return nil, err
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Error creating Kubernetes clientset: ", err)
		return nil, err
	}

	return clientset, nil
}

func (cs *controllerServer) DeleteVolume(_ context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
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

func getCredentials(secretMap map[string]string) (*s3client.ObjectStorageCredentials, error) {
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

func getCredentialsCustom(ctx context.Context, secretName, secretNamespace string, k8sClient *kubernetes.Clientset) (accessKey string, secretKey string, apiKey string, kpRootKeyCrn string, err error) {
	secrets, err := k8sClient.CoreV1().Secrets(secretNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", "", "", fmt.Errorf("cannot retrieve secret %s: %v", secretName, err)
	}
	klog.Info("secret retrieved: \n", secrets)

	if strings.TrimSpace(string(secrets.Type)) != "cos-s3-csi-driver" {
		return "", "", "", "", fmt.Errorf("Wrong Secret Type. Provided secret of type %s. Expected type %s", string(secrets.Type), "cos-s3-csi-driver")
	}

	//var serviceInstanceID string

	apiKey, err = parseSecret(secrets, "apiKey")
	if err != nil {
		klog.Info("api key found: \n", apiKey)
		accessKey, err = parseSecret(secrets, "accessKey")
		if err != nil {
			return "", "", "", "", err
		}
		klog.Info("accessKey found: \n", accessKey)

		secretKey, err = parseSecret(secrets, "secretKey")
		if err != nil {
			return "", "", "", "", err
		}
		klog.Info("secretKey found: \n", secretKey)
	} else {
		//serviceInstanceID, err = parseSecret(secrets, "service-instance-id")
		return "", "", "", "", err
	}

	if bytesVal, ok := secrets.Data["kpRootKeyCRN"]; ok {
		kpRootKeyCrn = string(bytesVal)
	}

	return accessKey, secretKey, apiKey, kpRootKeyCrn, nil
}

func parseSecret(secret *v1.Secret, keyName string) (string, error) {
	klog.Infof("secret in parseSecret: %v", secret)
	klog.Infof("\nsecret.Data in parseSecret: %v", secret.Data)
	klog.Infof("keyName parseSecret: %v", keyName)
	klog.Infof("secret.Data for keyName: %v", secret.Data[keyName])
	bytesVal, ok := secret.Data[keyName]
	if !ok {
		return "", fmt.Errorf("%s secret missing", keyName)
	}
	return string(bytesVal), nil
}

func getTempBucketName(mounterType, volumeID string) string {
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
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range volumeCapabilities {
			volumeCap := csi.VolumeCapability_AccessMode{
				Mode: c,
			}
			if volumeCap.GetMode() == cap.AccessMode.GetMode() {
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
