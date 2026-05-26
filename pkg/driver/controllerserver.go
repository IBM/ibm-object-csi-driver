/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023, 2025 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/logger"
	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/aws/smithy-go"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
)

// Implements Controller csi.ControllerServer
type controllerServer struct {
	*S3Driver
	csi.UnimplementedControllerServer
	Stats      utils.StatsUtils
	cosSession s3client.ObjectStorageSessionFactory
	Logger     *zap.Logger
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Extract request ID from context (added by interceptor)
	reqID := requestid.FromContext(ctx)
	
	var (
		bucketName         string
		endPoint           string
		locationConstraint string
		kpRootKeyCrn       string
		pvcName            string
		pvcNamespace       string
		bucketVersioning   string
		quotaLimitEnabled  bool
	)

	logger.Info(ctx, cs.Logger, "CreateVolume started", zap.String("volume_name", req.GetName()))

	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
	if err != nil {
		logger.Error(ctx, cs.Logger, "Error modifying request", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in modifying requests %v", reqID, err))
	}
	logger.Debug(ctx, cs.Logger, "CreateVolume request details", zap.Any("request", modifiedRequest.(*csi.CreateVolumeRequest)))

	volumeName, err := sanitizeVolumeID(req.GetName())
	if err != nil {
		logger.Error(ctx, cs.Logger, "Error sanitizing volume ID", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in sanitizeVolumeID %v", reqID, err))
	}
	volumeID := volumeName
	if len(volumeID) == 0 {
		logger.Error(ctx, cs.Logger, "Volume name missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume name missing in request", reqID))
	}
	logger.Info(ctx, cs.Logger, "Processing volume creation", zap.String("volume_id", volumeID))

	caps := req.GetVolumeCapabilities()
	if caps == nil {
		logger.Error(ctx, cs.Logger, "Volume capabilities missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume Capabilities missing in request", reqID))
	}
	for _, cap := range caps {
		logger.Debug(ctx, cs.Logger, "Volume capability", zap.String("capability", cap.String()))
		if cap.GetBlock() != nil {
			logger.Error(ctx, cs.Logger, "Block volume not supported")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume type block Volume not supported", reqID))
		}
	}

	params := req.GetParameters()
	if params == nil {
		params = make(map[string]string)
	}
	logger.Info(ctx, cs.Logger, "CreateVolume parameters received", zap.Int("param_count", len(params)))

	secretMap := req.GetSecrets()
	logger.Info(ctx, cs.Logger, "Secrets received", zap.Int("secret_count", len(secretMap)))

	var customSecretName string
	if len(secretMap) == 0 {
		logger.Info(ctx, cs.Logger, "No secret in request, fetching custom secret from PVC annotations")

		pvcName = params[constants.PVCNameKey]
		pvcNamespace = params[constants.PVCNamespaceKey]

		if pvcName == "" {
			logger.Error(ctx, cs.Logger, "PVC name not specified")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] pvcName not specified, could not fetch the secret", reqID))
		}

		if pvcNamespace == "" {
			pvcNamespace = constants.DefaultNamespace
		}

		logger.Info(ctx, cs.Logger, "Fetching PVC", zap.String("pvc_name", pvcName), zap.String("namespace", pvcNamespace))
		pvcRes, err := cs.Stats.GetPVC(pvcName, pvcNamespace)
		if err != nil {
			logger.Error(ctx, cs.Logger, "PVC resource not found", zap.Error(err))
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] PVC resource not found %v", reqID, err))
		}

		logger.Debug(ctx, cs.Logger, "PVC annotations", zap.Any("annotations", pvcRes.Annotations))

		pvcAnnotations := pvcRes.Annotations

		customSecretName = pvcAnnotations[constants.SecretNameKey]
		secretNamespace := pvcAnnotations[constants.SecretNamespaceKey]

		if customSecretName == "" {
			logger.Error(ctx, cs.Logger, "Secret name annotation not found in PVC")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] secretName annotation 'cos.csi.driver/secret' not specified in the PVC annotations", reqID))
		}

		if secretNamespace == "" {
			logger.Warn(ctx, cs.Logger, "Secret namespace not specified in PVC annotations, using default namespace")
			secretNamespace = constants.DefaultNamespace
		}

		logger.Info(ctx, cs.Logger, "Fetching secret", zap.String("secret_name", customSecretName), zap.String("namespace", secretNamespace))
		secret, err := cs.Stats.GetSecret(customSecretName, secretNamespace)
		if err != nil {
			logger.Error(ctx, cs.Logger, "Secret resource not found", zap.Error(err))
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Secret resource not found %v", reqID, err))
		}

		secretMapCustom := parseCustomSecret(ctx, secret, cs.Logger)
		logger.Info(ctx, cs.Logger, "Custom secret parsed successfully", zap.Int("secret_param_count", len(secretMapCustom)))

		if objectPath, exists := secretMapCustom["objectPath"]; exists {
			logger.Info(ctx, cs.Logger, "ObjectPath found in secret", zap.String("volume_id", volumeID), zap.String("object_path", objectPath))
			params["objectPath"] = objectPath
		} else {
			logger.Info(ctx, cs.Logger, "No objectPath in secret, mounting bucket root", zap.String("volume_id", volumeID))
		}

		secretMap = secretMapCustom
	}
	if quotaLimitStr, ok := secretMap[constants.QuotaLimitKey]; ok && quotaLimitStr != "" {
		logger.Info(ctx, cs.Logger, "Quota limit parameter found", zap.String("quota_limit", quotaLimitStr))
		quotaLimitEnabled, err = strconv.ParseBool(quotaLimitStr)
		if err != nil {
			logger.Error(ctx, cs.Logger, "Invalid quota limit value", zap.String("value", quotaLimitStr), zap.Error(err))
			return nil, status.Error(codes.InvalidArgument,
				fmt.Sprintf("[%s] invalid quotaLimit value %q: must be 'true' or 'false'", reqID, quotaLimitStr))
		}

		if quotaLimitEnabled {
			if secretMap[constants.ResourceConfigApiKey] == "" {
				logger.Error(ctx, cs.Logger, "Resource config API key missing for quota limit")
				return nil, status.Error(codes.InvalidArgument,
					fmt.Sprintf("[%s] resourceConfigApiKey missing in secret, cannot set quota limit for bucket", reqID))
			}

			quotaBytes := req.GetCapacityRange().GetRequiredBytes()
			if quotaBytes <= 0 {
				logger.Error(ctx, cs.Logger, "Invalid storage size for quota limit", zap.Int64("bytes", quotaBytes))
				return nil, status.Error(codes.InvalidArgument,
					fmt.Sprintf("[%s] enable quotaLimit requested but no positive storage size requested in PVC", reqID))
			}
			logger.Info(ctx, cs.Logger, "Quota limit enabled", zap.Int64("quota_bytes", quotaBytes))
		}
	}

	endPoint = secretMap["cosEndpoint"]
	if endPoint == "" {
		endPoint = params["cosEndpoint"]
	} else {
		params["cosEndpoint"] = endPoint
	}
	if endPoint == "" {
		logger.Error(ctx, cs.Logger, "COS endpoint not specified")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] cosEndpoint unknown", reqID))
	}

	locationConstraint = secretMap["locationConstraint"]
	if locationConstraint == "" {
		locationConstraint = params["locationConstraint"]
	} else {
		params["locationConstraint"] = locationConstraint
	}
	if locationConstraint == "" {
		logger.Error(ctx, cs.Logger, "Location constraint not specified")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] locationConstraint unknown", reqID))
	}

	kpRootKeyCrn = secretMap["kpRootKeyCRN"]
	if kpRootKeyCrn != "" {
		logger.Info(ctx, cs.Logger, "Key Protect root key CRN provided for bucket encryption")
	}

	mounter := secretMap["mounter"]
	if mounter == "" {
		mounter = params["mounter"]
	} else {
		params["mounter"] = mounter
	}
	logger.Info(ctx, cs.Logger, "Mounter type configured", zap.String("mounter", mounter))

	bucketName = secretMap["bucketName"]

	// Check for bucketVersioning parameter
	if val, ok := secretMap[constants.BucketVersioning]; ok && val != "" {
		enable := strings.ToLower(strings.TrimSpace(val))
		if enable != "true" && enable != "false" {
			logger.Error(ctx, cs.Logger, "Invalid bucket versioning value in secret", zap.String("value", val))
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Invalid BucketVersioning value in secret: %s. Value set %s. Must be 'true' or 'false'", reqID, customSecretName, val))
		}
		bucketVersioning = enable
		logger.Info(ctx, cs.Logger, "Bucket versioning from secret", zap.String("versioning", bucketVersioning))
	} else if val, ok := params[constants.BucketVersioning]; ok && val != "" {
		enable := strings.ToLower(strings.TrimSpace(val))
		if enable != "true" && enable != "false" {
			logger.Error(ctx, cs.Logger, "Invalid bucket versioning value in storage class", zap.String("value", val))
			return nil, status.Error(codes.InvalidArgument,
				fmt.Sprintf("[%s] Invalid bucketVersioning value in storage class: %s. Must be 'true' or 'false'", reqID, val))
		}
		bucketVersioning = enable
		logger.Info(ctx, cs.Logger, "Bucket versioning from storage class", zap.String("versioning", bucketVersioning))
	}

	creds, err := getObjectStorageCredentialsFromSecret(ctx, secretMap, cs.iamEndpoint, cs.Logger)
	if err != nil {
		logger.Error(ctx, cs.Logger, "Error getting credentials from secret", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in getting credentials %v", reqID, err))
	}
	logger.Info(ctx, cs.Logger, "Creating object storage session",
		zap.String("endpoint", endPoint),
		zap.String("location_constraint", locationConstraint))
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)

	params["userProvidedBucket"] = "true"
	if bucketName != "" {
		// User Provided bucket. Check its existence and create if not present
		logger.Info(ctx, cs.Logger, "Bucket name provided", zap.String("bucket_name", bucketName))
		logger.Info(ctx, cs.Logger, "Checking if bucket already exists", zap.String("bucket_name", bucketName))
		if err := sess.CheckBucketAccess(ctx, bucketName); err != nil {
			logger.Info(ctx, cs.Logger, "Bucket not accessible, creating new bucket",
				zap.String("bucket_name", bucketName), zap.Error(err))
			err = createBucket(ctx, sess, bucketName, kpRootKeyCrn, cs.Logger)
			if err != nil {
				logger.Error(ctx, cs.Logger, "Failed to create bucket",
					zap.String("bucket_name", bucketName), zap.Error(err))
				return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("[%s] %v: %v", reqID, err, bucketName))
			}
			params["userProvidedBucket"] = "false"
			logger.Info(ctx, cs.Logger, "Created bucket successfully", zap.String("bucket_name", bucketName))
		}

		if quotaLimitEnabled {
			quotaBytes := req.GetCapacityRange().GetRequiredBytes()
			resConfApikey := secretMap[constants.ResourceConfigApiKey]

			logger.Info(ctx, cs.Logger, "Applying hard quota to bucket",
				zap.Int64("quota_bytes", quotaBytes), zap.String("bucket_name", bucketName))
			err = sess.UpdateQuotaLimit(ctx, quotaBytes, resConfApikey, bucketName, endPoint, creds.IAMEndpoint)
			if err != nil {
				logger.Error(ctx, cs.Logger, "Failed to set quota limit on bucket",
					zap.String("bucket_name", bucketName), zap.Error(err))
				if params["userProvidedBucket"] == "false" {
					if delErr := sess.DeleteBucket(ctx, bucketName); delErr != nil {
						logger.Error(ctx, cs.Logger, "Failed to delete bucket after quota limit failure",
							zap.String("bucket_name", bucketName), zap.Error(delErr))
					}
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set bucket quota limit: %v", reqID, err))
			}
			logger.Info(ctx, cs.Logger, "Successfully applied hard quota to bucket",
				zap.Int64("quota_bytes", quotaBytes), zap.String("bucket_name", bucketName))
		}

		if bucketVersioning != "" {
			enable := strings.ToLower(strings.TrimSpace(bucketVersioning)) == "true"
			logger.Info(ctx, cs.Logger, "Setting bucket versioning",
				zap.Bool("enable", enable), zap.String("bucket_name", bucketName))

			err := sess.SetBucketVersioning(ctx, bucketName, enable)
			if err != nil {
				logger.Error(ctx, cs.Logger, "Failed to set bucket versioning",
					zap.Bool("enable", enable), zap.String("bucket_name", bucketName), zap.Error(err))
				if params["userProvidedBucket"] == "false" {
					err1 := sess.DeleteBucket(ctx, bucketName)
					if err1 != nil {
						logger.Error(ctx, cs.Logger, "Failed to delete bucket after versioning failure",
							zap.String("bucket_name", bucketName), zap.Error(err1))
						return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] cannot set versioning: %v and cannot delete bucket %s: %v", reqID, err, bucketName, err1))
					}
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set versioning %t for bucket %s: %v", reqID, enable, bucketName, err))
			}
			logger.Info(ctx, cs.Logger, "Bucket versioning set successfully",
				zap.Bool("enable", enable), zap.String("bucket_name", bucketName))
		}

		params["bucketName"] = bucketName
	} else {
		// Generate random temp bucket name based on volume id
		logger.Info(ctx, cs.Logger, "Bucket name not provided, generating temp bucket")
		tempBucketName := getTempBucketName(ctx, mounter, volumeID, cs.Logger)
		if tempBucketName == "" {
			logger.Error(ctx, cs.Logger, "Unable to generate temp bucket name")
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("[%s] Unable to access the bucket: %v", reqID, tempBucketName))
		}
		logger.Info(ctx, cs.Logger, "Creating temp bucket", zap.String("temp_bucket_name", tempBucketName))
		err = createBucket(ctx, sess, tempBucketName, kpRootKeyCrn, cs.Logger)
		if err != nil {
			logger.Error(ctx, cs.Logger, "Failed to create temp bucket",
				zap.String("temp_bucket_name", tempBucketName), zap.Error(err))
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("[%s] %v: %v", reqID, err, tempBucketName))
		}

		if quotaLimitEnabled {
			quotaBytes := req.GetCapacityRange().GetRequiredBytes()
			resConfApikey := secretMap[constants.ResourceConfigApiKey]

			logger.Info(ctx, cs.Logger, "Applying hard quota to temp bucket",
				zap.Int64("quota_bytes", quotaBytes), zap.String("temp_bucket_name", tempBucketName))
			err = sess.UpdateQuotaLimit(ctx, quotaBytes, resConfApikey, tempBucketName, endPoint, creds.IAMEndpoint)
			if err != nil {
				logger.Error(ctx, cs.Logger, "Failed to set quota limit on temp bucket",
					zap.String("temp_bucket_name", tempBucketName), zap.Error(err))
				if delErr := sess.DeleteBucket(ctx, tempBucketName); delErr != nil {
					logger.Error(ctx, cs.Logger, "Failed to delete temp bucket after quota limit failure",
						zap.String("temp_bucket_name", tempBucketName), zap.Error(delErr))
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set bucket quota limit: %v", reqID, err))
			}
			logger.Info(ctx, cs.Logger, "Successfully applied hard quota to temp bucket",
				zap.Int64("quota_bytes", quotaBytes), zap.String("temp_bucket_name", tempBucketName))
		}

		if bucketVersioning != "" {
			enable := strings.ToLower(strings.TrimSpace(bucketVersioning)) == "true"
			logger.Info(ctx, cs.Logger, "Setting temp bucket versioning",
				zap.Bool("enable", enable), zap.String("temp_bucket_name", tempBucketName))

			err := sess.SetBucketVersioning(ctx, tempBucketName, enable)
			if err != nil {
				logger.Error(ctx, cs.Logger, "Failed to set temp bucket versioning",
					zap.Bool("enable", enable), zap.String("temp_bucket_name", tempBucketName), zap.Error(err))
				err1 := sess.DeleteBucket(ctx, tempBucketName)
				if err1 != nil {
					logger.Error(ctx, cs.Logger, "Failed to delete temp bucket after versioning failure",
						zap.String("temp_bucket_name", tempBucketName), zap.Error(err1))
					return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] cannot set versioning: %v and cannot delete temp bucket %s: %v", reqID, err, tempBucketName, err1))
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set versioning %t for temp bucket %s: %v", reqID, enable, tempBucketName, err))
			}
			logger.Info(ctx, cs.Logger, "Temp bucket versioning set successfully",
				zap.Bool("enable", enable), zap.String("temp_bucket_name", tempBucketName))
		}
		logger.Info(ctx, cs.Logger, "Created temp bucket successfully", zap.String("temp_bucket_name", tempBucketName))
		params["userProvidedBucket"] = "false"
		params["bucketName"] = tempBucketName
	}
	
	logger.Info(ctx, cs.Logger, "CreateVolume completed successfully",
		zap.String("volume_id", volumeID),
		zap.String("bucket_name", params["bucketName"]),
		zap.Int64("capacity_bytes", req.GetCapacityRange().GetRequiredBytes()))

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeID,
			CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
			VolumeContext: params,
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// Extract request ID from context
	reqID := requestid.FromContext(ctx)
	
	logger.Info(ctx, cs.Logger, "DeleteVolume started")
	
	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
	if err != nil {
		logger.Error(ctx, cs.Logger, "Error modifying request", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in modifying requests %v", reqID, err))
	}
	logger.Debug(ctx, cs.Logger, "DeleteVolume request details", zap.Any("request", modifiedRequest.(*csi.DeleteVolumeRequest)))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		logger.Error(ctx, cs.Logger, "Volume ID missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}
	logger.Info(ctx, cs.Logger, "Deleting volume", zap.String("volume_id", volumeID))
	
	secretMap := req.GetSecrets()
	logger.Info(ctx, cs.Logger, "Secrets received", zap.Int("secret_count", len(secretMap)))

	endPoint := secretMap["cosEndpoint"]
	locationConstraint := secretMap["locationConstraint"]

	if len(secretMap) == 0 {
		logger.Info(ctx, cs.Logger, "No secret in request, fetching from PV")

		pv, err := cs.Stats.GetPV(volumeID)
		if err != nil {
			logger.Error(ctx, cs.Logger, "Failed to get PV", zap.String("volume_id", volumeID), zap.Error(err))
			return nil, err
		}

		logger.Debug(ctx, cs.Logger, "PV resource retrieved", zap.Any("pv", pv))

		secretName := pv.Spec.CSI.NodePublishSecretRef.Name
		secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace

		if secretName == "" {
			logger.Error(ctx, cs.Logger, "Secret details not found in PV")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Secret details not found, could not fetch the secret", reqID))
		}

		if secretNamespace == "" {
			logger.Warn(ctx, cs.Logger, "Secret namespace not found, using default namespace")
			secretNamespace = constants.DefaultNamespace
		}

		endPoint = pv.Spec.CSI.VolumeAttributes["cosEndpoint"]
		locationConstraint = pv.Spec.CSI.VolumeAttributes["locationConstraint"]

		logger.Info(ctx, cs.Logger, "Secret details found",
			zap.String("secret_name", secretName),
			zap.String("secret_namespace", secretNamespace))

		secret, err := cs.Stats.GetSecret(secretName, secretNamespace)
		if err != nil {
			logger.Error(ctx, cs.Logger, "Secret resource not found", zap.Error(err))
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Secret resource not found %v", reqID, err))
		}

		secretMapCustom := parseCustomSecret(ctx, secret, cs.Logger)
		logger.Info(ctx, cs.Logger, "Custom secret parsed successfully", zap.Int("secret_param_count", len(secretMapCustom)))
		secretMap = secretMapCustom
	}

	creds, err := getObjectStorageCredentialsFromSecret(ctx, secretMap, cs.iamEndpoint, cs.Logger)
	if err != nil {
		logger.Error(ctx, cs.Logger, "Error getting credentials from secret", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in getting credentials %v", reqID, err))
	}

	logger.Info(ctx, cs.Logger, "Creating object storage session for deletion",
		zap.String("endpoint", endPoint),
		zap.String("location_constraint", locationConstraint))
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)

	bucketToDelete, err := cs.Stats.BucketToDelete(volumeID)
	if err != nil {
		logger.Warn(ctx, cs.Logger, "No bucket to delete or error getting bucket info", zap.Error(err))
		return &csi.DeleteVolumeResponse{}, nil
	}

	if bucketToDelete != "" {
		logger.Info(ctx, cs.Logger, "Deleting bucket", zap.String("bucket_name", bucketToDelete))
		err = sess.DeleteBucket(ctx, bucketToDelete)
		if err != nil {
			logger.Warn(ctx, cs.Logger, "Cannot delete bucket",
				zap.String("bucket_name", bucketToDelete), zap.Error(err))
		} else {
			logger.Info(ctx, cs.Logger, "Bucket deleted successfully", zap.String("bucket_name", bucketToDelete))
		}
	} else {
		logger.Info(ctx, cs.Logger, "No bucket to delete")
	}

	logger.Info(ctx, cs.Logger, "DeleteVolume completed successfully", zap.String("volume_id", volumeID))
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "ControllerPublishVolume request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "ControllerPublishVolume not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] ControllerPublishVolume", reqID))
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "ControllerUnpublishVolume request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "ControllerUnpublishVolume not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] ControllerUnpublishVolume", reqID))
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Info(ctx, cs.Logger, "ValidateVolumeCapabilities started")
	logger.Debug(ctx, cs.Logger, "ValidateVolumeCapabilities request", zap.Any("request", req))

	// Validate Arguments
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		logger.Error(ctx, cs.Logger, "Volume ID missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		logger.Error(ctx, cs.Logger, "Volume capabilities missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume capabilities missing in request", reqID))
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volCaps) {
		logger.Info(ctx, cs.Logger, "Volume capabilities are valid", zap.String("volume_id", volumeID))
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	} else {
		logger.Warn(ctx, cs.Logger, "Volume capabilities are invalid", zap.String("volume_id", volumeID))
	}

	logger.Info(ctx, cs.Logger, "ValidateVolumeCapabilities completed", zap.String("volume_id", volumeID))
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "ListVolumes request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "ListVolumes not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] ListVolumes", reqID))
}

func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "GetCapacity request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "GetCapacity not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] GetCapacity", reqID))
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	logger.Info(ctx, cs.Logger, "ControllerGetCapabilities started")
	logger.Debug(ctx, cs.Logger, "ControllerGetCapabilities request", zap.Any("request", req))
	
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
	
	logger.Info(ctx, cs.Logger, "ControllerGetCapabilities completed", zap.Int("capability_count", len(caps)))
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "CreateSnapshot request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "CreateSnapshot not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] CreateSnapshot", reqID))
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "DeleteSnapshot request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "DeleteSnapshot not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] DeleteSnapshot", reqID))
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "ListSnapshots request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "ListSnapshots not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] ListSnapshots", reqID))
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "ControllerExpandVolume request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "ControllerExpandVolume not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] ControllerExpandVolume", reqID))
}

func (cs *controllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "ControllerGetVolume request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "ControllerGetVolume not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] ControllerGetVolume", reqID))
}

func (cs *controllerServer) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	
	logger.Debug(ctx, cs.Logger, "ControllerModifyVolume request", zap.Any("request", req))
	logger.Info(ctx, cs.Logger, "ControllerModifyVolume not implemented")
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] ControllerModifyVolume", reqID))
}

func getObjectStorageCredentialsFromSecret(ctx context.Context, secretMap map[string]string, iamEP string, log *zap.Logger) (*s3client.ObjectStorageCredentials, error) {
	logger.Info(ctx, log, "Getting object storage credentials from secret")
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
		iamEndpoint = iamEP
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

func parseCustomSecret(ctx context.Context, secret *v1.Secret, log *zap.Logger) map[string]string {
	logger.Info(ctx, log, "Parsing custom secret")
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
		objectPath         string
		resConfApiKey      string
		quotaLimit         string
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

	if bytesVal, ok := secret.Data["objectPath"]; ok {
		objectPath = string(bytesVal)
	}

	if bytesVal, ok := secret.Data[constants.ResourceConfigApiKey]; ok {
		resConfApiKey = string(bytesVal)
	}
	if bytesVal, ok := secret.Data[constants.QuotaLimitKey]; ok {
		quotaLimit = string(bytesVal)
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
	secretMapCustom["objectPath"] = objectPath
	secretMapCustom[constants.ResourceConfigApiKey] = resConfApiKey
	secretMapCustom[constants.QuotaLimitKey] = quotaLimit

	return secretMapCustom
}

func getTempBucketName(ctx context.Context, mounterType, volumeID string, log *zap.Logger) string {
	logger.Info(ctx, log, "Getting temp bucket name", zap.String("mounter_type", mounterType))
	currentTime := time.Now()
	timestamp := currentTime.Format("20060102150405")

	name := fmt.Sprintf("%s%s-%s", mounterType, timestamp, volumeID)
	return name
}

func createBucket(ctx context.Context, sess s3client.ObjectStorageSession, bucketName, kpRootKeyCrn string, log *zap.Logger) error {
	msg, err := sess.CreateBucket(ctx, bucketName, kpRootKeyCrn)
	if msg != "" {
		logger.Info(ctx, log, "Bucket creation info", zap.String("message", msg))
	}
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "BucketAlreadyExists" {
			logger.Warn(ctx, log, "Bucket already exists", zap.String("bucket", bucketName))
		} else {
			logger.Error(ctx, log, "Unable to create bucket", zap.String("bucket", bucketName), zap.Error(err))
			return errors.New("unable to create the bucket")
		}
	}
	if err := sess.CheckBucketAccess(ctx, bucketName); err != nil {
		logger.Error(ctx, log, "Unable to access bucket", zap.String("bucket", bucketName), zap.Error(err))
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
