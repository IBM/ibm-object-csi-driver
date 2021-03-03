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
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	commonError "github.ibm.com/alchemy-containers/ibm-csi-common/pkg/messages"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/metrics"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"strings"
	"time"
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

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-ControllerPublishVolume", zap.Reflect("Request", *req))
	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "ControllerPublishVolume")
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-ControllerUnPublishVolume", zap.Reflect("Request", *req))
	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "ControllerUnPublishVolume")
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
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	ctx = context.WithValue(ctx, provider.RequestID, requestID)
	ctxLogger.Info("CSIControllerServer-CreateVolume... ", zap.Reflect("Request", *req))
	defer metrics.UpdateDurationFromStart(ctxLogger, "CreateVolume", time.Now())

	volumeName := sanitizeVolumeID(req.GetName())
	volumeID := volumeName

	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		ctxLogger.Info("invalid create volume req:", zap.Reflect("Request", *req))
		return nil, err
	}

	// Check arguments
	if len(volumeID) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.MissingVolumeName, requestID, nil)
	}
	caps := req.GetVolumeCapabilities()
	if caps == nil {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoVolumeCapabilities, requestID, nil)
	}
	for _, cap := range caps {
		if cap.GetBlock() != nil {
			return nil, commonError.GetCSIError(ctxLogger, commonError.VolumeCapabilitiesNotSupported, requestID, nil)
		}
	}
	// Check for already existing volume name
	if _, err := getVolumeByName(req.GetName()); err == nil {
		ctxLogger.Info("Volume already exists", zap.Reflect("ExistingVolume", req.GetName()))
		return nil, commonError.GetCSIError(ctxLogger, commonError.VolumeAlreadyExists, requestID, err, req.GetName())
	}

	// Check for maximum available capacity
	capacity := int64(req.GetCapacityRange().GetRequiredBytes())
	if capacity >= maxStorageCapacity {
		return nil, status.Errorf(codes.OutOfRange, fmt.Sprintf("Requested capacity %d exceeds maximum allowed %d", capacity, maxStorageCapacity))
	}

	ctxLogger.Info("Got a request to create volume:", zap.Reflect("volumeID", volumeID))
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
		ctxLogger.Error("CreateVolume Unable to initialize backend S3 Client")
		return nil, status.Error(codes.PermissionDenied, "Unable to initialize backend S3 Clinet")
	}

	if err := cs.s3Driver.s3client.CheckBucketAccess(bucketName); err != nil {
		ctxLogger.Error("CreateVolume Unable to access the bucket:", zap.Error(err))
		return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to to access the bucket: %v", bucketName))
	}

	params["cos-endpoint"] = endPoint
	params["regn-class"] = regnClass
	params["bucket-name"] = bucketName
	params["obj-path"] = objPath

	ctxLogger.Info("create volume:", zap.Reflect("volumeID", volumeID))

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
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	ctx = context.WithValue(ctx, provider.RequestID, requestID)
	defer metrics.UpdateDurationFromStart(ctxLogger, "DeleteVolume", time.Now())
	ctxLogger.Info("CSIControllerServer-DeleteVolume... ", zap.Reflect("Request", *req))

	// Validate arguments
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.EmptyVolumeID, requestID, nil)
	}

	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		ctxLogger.Info("Invalid delete volume req", zap.Reflect("Request", *req))
		return nil, err
	}
	ctxLogger.Info("Deleting volume", zap.Reflect("VolumeID", volumeID))
	ctxLogger.Info("deleting volume", zap.Reflect("volumeID", volumeID))
	//path := provisionRoot + volumeID
	//os.RemoveAll(path)
	delete(s3CosVolumes, volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	ctx = context.WithValue(ctx, provider.RequestID, requestID)
	ctxLogger.Info("CSIControllerServer-ValidateVolumeCapabilities", zap.Reflect("Request", *req))

	// Validate Arguments
	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoVolumeCapabilities, requestID, nil)
	}

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.EmptyVolumeID, requestID, nil)
	}

	if _, ok := s3CosVolumes[req.GetVolumeId()]; !ok {
		return nil, status.Error(codes.NotFound, "Volume does not exist")
	}
	return cs.DefaultControllerServer.ValidateVolumeCapabilities(ctx, req)

}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-CreateSnapshot", zap.Reflect("Request", *req))

	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "CreateSnapshot")

}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-DeleteSnapshot", zap.Reflect("Request", *req))

	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "DeleteSnapshot")
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-ListSnapshots", zap.Reflect("Request", *req))

	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "ListSnapshots")
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-ControllerExpandVolume", zap.Reflect("Request", *req))
	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "ControllerExpandVolume")
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
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-GetCapacity", zap.Reflect("Request", *req))
	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "GetCapacity")
}

//ListVolumes
func (csiCS *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	// populate requestID in the context
	_ = context.WithValue(ctx, provider.RequestID, requestID)

	ctxLogger.Info("CSIControllerServer-ListVolumes", zap.Reflect("Request", *req))
	return nil, commonError.GetCSIError(ctxLogger, commonError.MethodUnimplemented, requestID, nil, "ListVolumes")
}
