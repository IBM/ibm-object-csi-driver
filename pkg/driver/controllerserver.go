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

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Extract request ID from context (added by interceptor)
	reqID := requestid.FromContext(ctx)
	log := cs.Logger.With(zap.String("request_id", reqID))
	
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

	log.Info("CreateVolume started", zap.String("volume_name", req.GetName()))

	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
	if err != nil {
		logger.Error(ctx, cs.Logger, "Error modifying request", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in modifying requests %v", reqID, err))
	}
	log.Debug("CreateVolume request details", zap.Any("request", modifiedRequest.(*csi.CreateVolumeRequest)))

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
	log.Info("Processing volume creation", zap.String("volume_id", volumeID))

	caps := req.GetVolumeCapabilities()
	if caps == nil {
		logger.Error(ctx, cs.Logger, "Volume capabilities missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume Capabilities missing in request", reqID))
	}
	for _, cap := range caps {
		log.Debug("Volume capability", zap.String("capability", cap.String()))
		if cap.GetBlock() != nil {
			logger.Error(ctx, cs.Logger, "Block volume not supported")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume type block Volume not supported", reqID))
		}
	}

	params := req.GetParameters()
	if params == nil {
		params = make(map[string]string)
	}
	log.Info("CreateVolume parameters received", zap.Int("param_count", len(params)))

	secretMap := req.GetSecrets()
	log.Info("Secrets received", zap.Int("secret_count", len(secretMap)))

	var customSecretName string
	if len(secretMap) == 0 {
		log.Info("No secret in request, fetching custom secret from PVC annotations")

		pvcName = params[constants.PVCNameKey]
		pvcNamespace = params[constants.PVCNamespaceKey]

		if pvcName == "" {
			logger.Error(ctx, cs.Logger, "PVC name not specified")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] pvcName not specified, could not fetch the secret", reqID))
		}

		if pvcNamespace == "" {
			pvcNamespace = constants.DefaultNamespace
		}

		log.Info("Fetching PVC", zap.String("pvc_name", pvcName), zap.String("namespace", pvcNamespace))
		pvcRes, err := cs.Stats.GetPVC(pvcName, pvcNamespace)
		if err != nil {
			logger.Error(ctx, cs.Logger, "PVC resource not found", zap.Error(err))
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] PVC resource not found %v", reqID, err))
		}

		log.Debug("PVC annotations", zap.Any("annotations", pvcRes.Annotations))

		pvcAnnotations := pvcRes.Annotations

		customSecretName = pvcAnnotations[constants.SecretNameKey]
		secretNamespace := pvcAnnotations[constants.SecretNamespaceKey]

		if customSecretName == "" {
			logger.Error(ctx, cs.Logger, "Secret name annotation not found in PVC")
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] secretName annotation 'cos.csi.driver/secret' not specified in the PVC annotations", reqID))
		}

		if secretNamespace == "" {
			log.Warn("Secret namespace not specified in PVC annotations, using default namespace")
			secretNamespace = constants.DefaultNamespace
		}

		log.Info("Fetching secret", zap.String("secret_name", customSecretName), zap.String("namespace", secretNamespace))
		secret, err := cs.Stats.GetSecret(customSecretName, secretNamespace)
		if err != nil {
			logger.Error(ctx, cs.Logger, "Secret resource not found", zap.Error(err))
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Secret resource not found %v", reqID, err))
		}

		secretMapCustom := parseCustomSecret(secret)
		log.Info("Custom secret parsed successfully", zap.Int("secret_param_count", len(secretMapCustom)))

		if objectPath, exists := secretMapCustom["objectPath"]; exists {
			log.Info("ObjectPath found in secret", zap.String("volume_id", volumeID), zap.String("object_path", objectPath))
			params["objectPath"] = objectPath
		} else {
			log.Info("No objectPath in secret, mounting bucket root", zap.String("volume_id", volumeID))
		}

		secretMap = secretMapCustom
	}
	if quotaLimitStr, ok := secretMap[constants.QuotaLimitKey]; ok && quotaLimitStr != "" {
		log.Info(fmt.Sprintf("[%s] Quota limit parameter found", reqID), zap.String("quota_limit", quotaLimitStr))
		quotaLimitEnabled, err = strconv.ParseBool(quotaLimitStr)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Invalid quota limit value", reqID), zap.String("value", quotaLimitStr), zap.Error(err))
			return nil, status.Error(codes.InvalidArgument,
				fmt.Sprintf("[%s] invalid quotaLimit value %q: must be 'true' or 'false'", reqID, quotaLimitStr))
		}

		if quotaLimitEnabled {
			if secretMap[constants.ResourceConfigApiKey] == "" {
				log.Error(fmt.Sprintf("[%s] Resource config API key missing for quota limit", reqID))
				return nil, status.Error(codes.InvalidArgument,
					fmt.Sprintf("[%s] resourceConfigApiKey missing in secret, cannot set quota limit for bucket", reqID))
			}

			quotaBytes := req.GetCapacityRange().GetRequiredBytes()
			if quotaBytes <= 0 {
				log.Error(fmt.Sprintf("[%s] Invalid storage size for quota limit", reqID), zap.Int64("bytes", quotaBytes))
				return nil, status.Error(codes.InvalidArgument,
					fmt.Sprintf("[%s] enable quotaLimit requested but no positive storage size requested in PVC", reqID))
			}
			log.Info(fmt.Sprintf("[%s] Quota limit enabled", reqID), zap.Int64("quota_bytes", quotaBytes))
		}
	}

	endPoint = secretMap["cosEndpoint"]
	if endPoint == "" {
		endPoint = params["cosEndpoint"]
	} else {
		params["cosEndpoint"] = endPoint
	}
	if endPoint == "" {
		log.Error(fmt.Sprintf("[%s] COS endpoint not specified", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] cosEndpoint unknown", reqID))
	}

	locationConstraint = secretMap["locationConstraint"]
	if locationConstraint == "" {
		locationConstraint = params["locationConstraint"]
	} else {
		params["locationConstraint"] = locationConstraint
	}
	if locationConstraint == "" {
		log.Error(fmt.Sprintf("[%s] Location constraint not specified", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] locationConstraint unknown", reqID))
	}

	kpRootKeyCrn = secretMap["kpRootKeyCRN"]
	if kpRootKeyCrn != "" {
		log.Info(fmt.Sprintf("[%s] Key Protect root key CRN provided for bucket encryption", reqID))
	}

	mounter := secretMap["mounter"]
	if mounter == "" {
		mounter = params["mounter"]
	} else {
		params["mounter"] = mounter
	}
	log.Info(fmt.Sprintf("[%s] Mounter type configured", reqID), zap.String("mounter", mounter))

	bucketName = secretMap["bucketName"]

	// Check for bucketVersioning parameter
	if val, ok := secretMap[constants.BucketVersioning]; ok && val != "" {
		enable := strings.ToLower(strings.TrimSpace(val))
		if enable != "true" && enable != "false" {
			log.Error(fmt.Sprintf("[%s] Invalid bucket versioning value in secret", reqID), zap.String("value", val))
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Invalid BucketVersioning value in secret: %s. Value set %s. Must be 'true' or 'false'", reqID, customSecretName, val))
		}
		bucketVersioning = enable
		log.Info(fmt.Sprintf("[%s] Bucket versioning from secret", reqID), zap.String("versioning", bucketVersioning))
	} else if val, ok := params[constants.BucketVersioning]; ok && val != "" {
		enable := strings.ToLower(strings.TrimSpace(val))
		if enable != "true" && enable != "false" {
			log.Error(fmt.Sprintf("[%s] Invalid bucket versioning value in storage class", reqID), zap.String("value", val))
			return nil, status.Error(codes.InvalidArgument,
				fmt.Sprintf("[%s] Invalid bucketVersioning value in storage class: %s. Must be 'true' or 'false'", reqID, val))
		}
		bucketVersioning = enable
		log.Info(fmt.Sprintf("[%s] Bucket versioning from storage class", reqID), zap.String("versioning", bucketVersioning))
	}

	creds, err := getObjectStorageCredentialsFromSecret(secretMap, cs.iamEndpoint)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] Error getting credentials from secret", reqID), zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in getting credentials %v", reqID, err))
	}
	log.Info(fmt.Sprintf("[%s] Creating object storage session", reqID),
		zap.String("endpoint", endPoint),
		zap.String("location_constraint", locationConstraint))
	sess := cs.cosSession.NewObjectStorageSession(endPoint, locationConstraint, creds, cs.Logger)

	params["userProvidedBucket"] = "true"
	if bucketName != "" {
		// User Provided bucket. Check its existence and create if not present
		log.Info(fmt.Sprintf("[%s] Bucket name provided", reqID), zap.String("bucket_name", bucketName))
		log.Info(fmt.Sprintf("[%s] Checking if bucket already exists", reqID), zap.String("bucket_name", bucketName))
		if err := sess.CheckBucketAccess(bucketName); err != nil {
			log.Info(fmt.Sprintf("[%s] Bucket not accessible, creating new bucket", reqID),
				zap.String("bucket_name", bucketName), zap.Error(err))
			err = createBucket(sess, bucketName, kpRootKeyCrn)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] Failed to create bucket", reqID),
					zap.String("bucket_name", bucketName), zap.Error(err))
				return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("[%s] %v: %v", reqID, err, bucketName))
			}
			params["userProvidedBucket"] = "false"
			log.Info(fmt.Sprintf("[%s] Created bucket successfully", reqID), zap.String("bucket_name", bucketName))
		}

		if quotaLimitEnabled {
			quotaBytes := req.GetCapacityRange().GetRequiredBytes()
			resConfApikey := secretMap[constants.ResourceConfigApiKey]

			log.Info(fmt.Sprintf("[%s] Applying hard quota to bucket", reqID),
				zap.Int64("quota_bytes", quotaBytes), zap.String("bucket_name", bucketName))
			err = sess.UpdateQuotaLimit(quotaBytes, resConfApikey, bucketName, endPoint, creds.IAMEndpoint)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] Failed to set quota limit on bucket", reqID),
					zap.String("bucket_name", bucketName), zap.Error(err))
				if params["userProvidedBucket"] == "false" {
					if delErr := sess.DeleteBucket(bucketName); delErr != nil {
						log.Error(fmt.Sprintf("[%s] Failed to delete bucket after quota limit failure", reqID),
							zap.String("bucket_name", bucketName), zap.Error(delErr))
					}
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set bucket quota limit: %v", reqID, err))
			}
			log.Info(fmt.Sprintf("[%s] Successfully applied hard quota to bucket", reqID),
				zap.Int64("quota_bytes", quotaBytes), zap.String("bucket_name", bucketName))
		}

		if bucketVersioning != "" {
			enable := strings.ToLower(strings.TrimSpace(bucketVersioning)) == "true"
			log.Info(fmt.Sprintf("[%s] Setting bucket versioning", reqID),
				zap.Bool("enable", enable), zap.String("bucket_name", bucketName))

			err := sess.SetBucketVersioning(bucketName, enable)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] Failed to set bucket versioning", reqID),
					zap.Bool("enable", enable), zap.String("bucket_name", bucketName), zap.Error(err))
				if params["userProvidedBucket"] == "false" {
					err1 := sess.DeleteBucket(bucketName)
					if err1 != nil {
						log.Error(fmt.Sprintf("[%s] Failed to delete bucket after versioning failure", reqID),
							zap.String("bucket_name", bucketName), zap.Error(err1))
						return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] cannot set versioning: %v and cannot delete bucket %s: %v", reqID, err, bucketName, err1))
					}
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set versioning %t for bucket %s: %v", reqID, enable, bucketName, err))
			}
			log.Info(fmt.Sprintf("[%s] Bucket versioning set successfully", reqID),
				zap.Bool("enable", enable), zap.String("bucket_name", bucketName))
		}

		params["bucketName"] = bucketName
	} else {
		// Generate random temp bucket name based on volume id
		log.Info(fmt.Sprintf("[%s] Bucket name not provided, generating temp bucket", reqID))
		tempBucketName := getTempBucketName(mounter, volumeID)
		if tempBucketName == "" {
			log.Error(fmt.Sprintf("[%s] Unable to generate temp bucket name", reqID))
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("[%s] Unable to access the bucket: %v", reqID, tempBucketName))
		}
		log.Info(fmt.Sprintf("[%s] Creating temp bucket", reqID), zap.String("temp_bucket_name", tempBucketName))
		err = createBucket(sess, tempBucketName, kpRootKeyCrn)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Failed to create temp bucket", reqID),
				zap.String("temp_bucket_name", tempBucketName), zap.Error(err))
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("[%s] %v: %v", reqID, err, tempBucketName))
		}

		if quotaLimitEnabled {
			quotaBytes := req.GetCapacityRange().GetRequiredBytes()
			resConfApikey := secretMap[constants.ResourceConfigApiKey]

			log.Info(fmt.Sprintf("[%s] Applying hard quota to temp bucket", reqID),
				zap.Int64("quota_bytes", quotaBytes), zap.String("temp_bucket_name", tempBucketName))
			err = sess.UpdateQuotaLimit(quotaBytes, resConfApikey, tempBucketName, endPoint, creds.IAMEndpoint)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] Failed to set quota limit on temp bucket", reqID),
					zap.String("temp_bucket_name", tempBucketName), zap.Error(err))
				if delErr := sess.DeleteBucket(tempBucketName); delErr != nil {
					log.Error(fmt.Sprintf("[%s] Failed to delete temp bucket after quota limit failure", reqID),
						zap.String("temp_bucket_name", tempBucketName), zap.Error(delErr))
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set bucket quota limit: %v", reqID, err))
			}
			log.Info(fmt.Sprintf("[%s] Successfully applied hard quota to temp bucket", reqID),
				zap.Int64("quota_bytes", quotaBytes), zap.String("temp_bucket_name", tempBucketName))
		}

		if bucketVersioning != "" {
			enable := strings.ToLower(strings.TrimSpace(bucketVersioning)) == "true"
			log.Info(fmt.Sprintf("[%s] Setting temp bucket versioning", reqID),
				zap.Bool("enable", enable), zap.String("temp_bucket_name", tempBucketName))

			err := sess.SetBucketVersioning(tempBucketName, enable)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] Failed to set temp bucket versioning", reqID),
					zap.Bool("enable", enable), zap.String("temp_bucket_name", tempBucketName), zap.Error(err))
				err1 := sess.DeleteBucket(tempBucketName)
				if err1 != nil {
					log.Error(fmt.Sprintf("[%s] Failed to delete temp bucket after versioning failure", reqID),
						zap.String("temp_bucket_name", tempBucketName), zap.Error(err1))
					return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] cannot set versioning: %v and cannot delete temp bucket %s: %v", reqID, err, tempBucketName, err1))
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] failed to set versioning %t for temp bucket %s: %v", reqID, enable, tempBucketName, err))
			}
			log.Info(fmt.Sprintf("[%s] Temp bucket versioning set successfully", reqID),
				zap.Bool("enable", enable), zap.String("temp_bucket_name", tempBucketName))
		}
		log.Info(fmt.Sprintf("[%s] Created temp bucket successfully", reqID), zap.String("temp_bucket_name", tempBucketName))
		params["userProvidedBucket"] = "false"
		params["bucketName"] = tempBucketName
	}
	
	log.Info(fmt.Sprintf("[%s] CreateVolume completed successfully", reqID),
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

	creds, err := getObjectStorageCredentialsFromSecret(secretMap, cs.iamEndpoint)
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

func getObjectStorageCredentialsFromSecret(secretMap map[string]string, iamEP string) (*s3client.ObjectStorageCredentials, error) {
	klog.Infof("- getObjectStorageCredentialsFromSecret-")
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "BucketAlreadyExists" {
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
