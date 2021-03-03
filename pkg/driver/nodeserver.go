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
	"fmt"
	"github.com/IBM/satellite-object-storage-plugin/pkg/mounter"
	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	commonError "github.ibm.com/alchemy-containers/ibm-csi-common/pkg/messages"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/metrics"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
	"os"
	"sync"
	"time"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
	mux sync.Mutex
	*s3Driver
}

var (
	mounterObj  mounter.Mounter
	newmounter  = mounter.NewMounter
	fuseunmount = mounter.FuseUnmount
)

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	var (
		val       string
		check     bool
		accessKey string
		secretKey string
	)

	publishContext := req.GetPublishContext()
	controlleRequestID := publishContext[PublishInfoRequestID]
	ctxLogger, requestID := utils.GetContextLoggerWithRequestID(ctx, false, &controlleRequestID)
	ctxLogger.Info("CSINodeServer-NodePublishVolume...", zap.Reflect("Request", *req))
	metrics.UpdateDurationFromStart(ctxLogger, "NodePublishVolume", time.Now())

	ns.mux.Lock()
	defer ns.mux.Unlock()

	volumeID := req.GetVolumeId()

	if len(volumeID) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.EmptyVolumeID, requestID, nil)
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoTargetPath, requestID, nil)
	}

	stagingTargetPath := req.GetStagingTargetPath()

	if len(stagingTargetPath) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoStagingTargetPath, requestID, nil)
	}

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoVolumeCapabilities, requestID, nil)
	}

	notMnt, err := checkMount(targetPath)
	if err != nil {
		ctxLogger.Error(fmt.Sprintf("Can not validate target mount point: %s %v", targetPath, err))
		return nil, commonError.GetCSIError(ctxLogger, commonError.MountPointValidateError, requestID, err, targetPath)
	}
	if !notMnt {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	deviceID := ""
	if req.GetPublishContext() != nil {
		deviceID = req.GetPublishContext()[deviceID]
	}

	readOnly := req.GetReadonly()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

	ctxLogger.Info(fmt.Sprintf("target %v\ndevice %v\nreadonly %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
		targetPath, deviceID, readOnly, volumeID, attrib, mountFlags))

	secretMap := req.GetSecrets()
	if val, check = secretMap["access-key"]; check {
		accessKey = val
	}
	if val, check = secretMap["secret-key"]; check {
		secretKey = val
	}
	fmt.Println("CreateVolume VolumeContext:\n\t", attrib)
	fmt.Println("CreateVolume Secrets:\n\t", secretMap)

	//func newMounter(mounter string, bucket string, objpath string, endpoint string, region string, keys string) (Mounter, error) {
	if mounterObj, err = newmounter("s3fs",
		attrib["bucket-name"], attrib["obj-path"],
		attrib["cos-endpoint"], attrib["regn-class"],
		fmt.Sprintf("%s:%s", accessKey, secretKey)); err != nil {
		return nil, err
	}

	ctxLogger.Info("-NodePublishVolume-: Mount")

	if err = mounterObj.Mount("", targetPath); err != nil {
		return nil, err
	}

	ctxLogger.Info(fmt.Sprintf("s3: bucket %s successfuly mounted to %s", attrib["bucket-name"], targetPath))
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	ctxLogger.Info("CSINodeServer-NodeUnpublishVolume...", zap.Reflect("Request", *req))
	metrics.UpdateDurationFromStart(ctxLogger, "NodeUnpublishVolume", time.Now())
	ns.mux.Lock()
	defer ns.mux.Unlock()

	volumeID := req.GetVolumeId()
	targetPath := req.GetTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.EmptyVolumeID, requestID, nil)
	}
	if len(targetPath) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoTargetPath, requestID, nil)
	}
	ctxLogger.Info("Unmounting  target path", zap.String("targetPath", targetPath))

	if err := fuseunmount(targetPath); err != nil {
		return nil, commonError.GetCSIError(ctxLogger, commonError.UnmountFailed, requestID, err, targetPath)
	}
	ctxLogger.Info("Successfully unmounted  target path", zap.String("targetPath", targetPath))

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	publishContext := req.GetPublishContext()
	controlleRequestID := publishContext[PublishInfoRequestID]
	ctxLogger, requestID := utils.GetContextLoggerWithRequestID(ctx, false, &controlleRequestID)
	ctxLogger.Info("CSINodeServer-NodeStageVolume...", zap.Reflect("Request", *req))
	metrics.UpdateDurationFromStart(ctxLogger, "NodeStageVolume", time.Now())

	ns.mux.Lock()
	defer ns.mux.Unlock()

	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.EmptyVolumeID, requestID, nil)
	}

	if len(stagingTargetPath) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoStagingTargetPath, requestID, nil)
	}

	if req.VolumeCapability == nil {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoVolumeCapabilities, requestID, nil)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	ctxLogger.Info("CSINodeServer-NodeUnstageVolume ... ", zap.Reflect("Request", *req))
	metrics.UpdateDurationFromStart(ctxLogger, "NodeUnstageVolume", time.Now())
	ns.mux.Lock()
	defer ns.mux.Unlock()

	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.EmptyVolumeID, requestID, nil)
	}
	if len(stagingTargetPath) == 0 {
		return nil, commonError.GetCSIError(ctxLogger, commonError.NoStagingTargetPath, requestID, nil)
	}
	ctxLogger.Info("Unmounting staging target path", zap.String("stagingTargetPath", stagingTargetPath))
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodeGetCapabilities returns the supported capabilities of the node server
func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// currently there is a single NodeServer capability according to the spec
	nscap := &csi.NodeServiceCapability{
		Type: &csi.NodeServiceCapability_Rpc{
			Rpc: &csi.NodeServiceCapability_RPC{
				Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
			},
		},
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			nscap,
		},
	}, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, status.Error(codes.Unimplemented, "NodeExpandVolume is not implemented")
}

func checkMount(targetPath string) (bool, error) {
	notMnt, err := mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return false, err
			}
			notMnt = true
		} else {
			return false, err
		}
	}
	return notMnt, nil
}
