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
	"context"
	"fmt"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/logger"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Implements Node Server csi.NodeServer
type nodeServer struct {
	*S3Driver
	csi.UnimplementedNodeServer
	Stats utils.StatsUtils
	NodeServerConfig
	Mounter      mounter.NewMounterFactory
	MounterUtils mounterUtils.MounterUtils
}

type NodeServerConfig struct {
	MaxVolumesPerNode int64
	Region            string
	Zone              string
	NodeID            string
	TLSCipherSuite    string
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)

	logger.Info(ctx, ns.logger, "NodeStageVolume started")
	logger.Debug(ctx, ns.logger, "NodeStageVolume request", zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		logger.Error(ctx, ns.logger, "Volume ID missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		logger.Error(ctx, ns.logger, "Target path missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}

	logger.Info(ctx, ns.logger, "NodeStageVolume completed",
		zap.String("volume_id", volumeID),
		zap.String("staging_target_path", stagingTargetPath))
	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)

	logger.Info(ctx, ns.logger, "NodeUnstageVolume started")
	logger.Debug(ctx, ns.logger, "NodeUnstageVolume request", zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		logger.Error(ctx, ns.logger, "Volume ID missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		logger.Error(ctx, ns.logger, "Target path missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}

	logger.Info(ctx, ns.logger, "NodeUnstageVolume completed",
		zap.String("volume_id", volumeID),
		zap.String("staging_target_path", stagingTargetPath))
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)

	logger.Info(ctx, ns.logger, "NodePublishVolume started")

	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
	if err != nil {
		logger.Error(ctx, ns.logger, "Error modifying request", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in modifying requests %v", reqID, err))
	}
	logger.Debug(ctx, ns.logger, "NodePublishVolume request", zap.Any("request", modifiedRequest.(*csi.NodePublishVolumeRequest)))

	volumeMountGroup := req.GetVolumeCapability().GetMount().GetVolumeMountGroup()
	logger.Debug(ctx, ns.logger, "Volume mount group", zap.String("volume_mount_group", volumeMountGroup))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		logger.Error(ctx, ns.logger, "Volume ID missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		logger.Error(ctx, ns.logger, "Target path missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}

	if req.GetVolumeCapability() == nil {
		logger.Error(ctx, ns.logger, "Volume capability missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume capability missing in request", reqID))
	}

	logger.Info(ctx, ns.logger, "Checking mount point", zap.String("target_path", targetPath))
	err = ns.Stats.CheckMount(targetPath)
	if err != nil {
		logger.Error(ctx, ns.logger, "Cannot validate target mount point",
			zap.String("target_path", targetPath), zap.Error(err))
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] %v", reqID, err.Error()))
	}

	deviceID := ""
	if req.GetPublishContext() != nil {
		deviceID = req.GetPublishContext()[deviceID]
	}

	readOnly := req.GetReadonly()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()
	logger.Debug(ctx, ns.logger, "NodePublishVolume parameters",
		zap.String("target_path", targetPath),
		zap.String("device_id", deviceID),
		zap.Bool("readonly", readOnly),
		zap.String("volume_id", volumeID),
		zap.Any("attributes", attrib),
		zap.Strings("mount_flags", mountFlags))

	secretMap := req.GetSecrets()
	logger.Info(ctx, ns.logger, "Secrets received", zap.Int("secret_count", len(secretMap)))

	secretMapCopy := make(map[string]string)
	for k, v := range secretMap {
		if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
			secretMapCopy[k] = "xxxxxxx"
			continue
		}
		secretMapCopy[k] = v
	}
	logger.Debug(ctx, ns.logger, "Secret map (sanitized)", zap.Any("secret_map", secretMapCopy))

	if volumeMountGroup != "" {
		secretMap["gid"] = volumeMountGroup
		logger.Debug(ctx, ns.logger, "Added volume mount group to secrets", zap.String("gid", volumeMountGroup))
	}

	if len(secretMap["cosEndpoint"]) == 0 {
		secretMap["cosEndpoint"] = attrib["cosEndpoint"]
		logger.Debug(ctx, ns.logger, "Using cosEndpoint from attributes", zap.String("cos_endpoint", secretMap["cosEndpoint"]))
	}

	if len(secretMap["locationConstraint"]) == 0 {
		secretMap["locationConstraint"] = attrib["locationConstraint"]
		logger.Debug(ctx, ns.logger, "Using locationConstraint from attributes", zap.String("location_constraint", secretMap["locationConstraint"]))
	}

	if len(secretMap["cosEndpoint"]) == 0 {
		logger.Error(ctx, ns.logger, "S3 Service endpoint not provided")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] S3 Service endpoint not provided", reqID))
	}

	if len(secretMap["iamEndpoint"]) == 0 {
		secretMap["iamEndpoint"] = ns.iamEndpoint
		logger.Debug(ctx, ns.logger, "Using default IAM endpoint", zap.String("iam_endpoint", ns.iamEndpoint))
	}

	// If bucket name wasn't provided by user, we use temp bucket created for volume.
	if secretMap["bucketName"] == "" {
		logger.Info(ctx, ns.logger, "Bucket name not provided, fetching from PV")
		tempBucketName, err := ns.Stats.GetBucketNameFromPV(volumeID)
		if err != nil {
			logger.Error(ctx, ns.logger, "Unable to fetch PV", zap.String("volume_id", volumeID), zap.Error(err))
			return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] %v", reqID, err.Error()))
		}

		if tempBucketName == "" {
			logger.Error(ctx, ns.logger, "Unable to fetch bucket name from PV")
			return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] unable to fetch bucket name from pv", reqID))
		}

		secretMap["bucketName"] = tempBucketName
		logger.Info(ctx, ns.logger, "Using bucket from PV", zap.String("bucket_name", tempBucketName))
	}

	var defaultParamsMap = map[string]string{
		constants.CipherSuitesKey: ns.TLSCipherSuite,
	}

	logger.Info(ctx, ns.logger, "Creating mounter object")
	mounterObj := ns.Mounter.NewMounter(attrib, secretMap, mountFlags, defaultParamsMap)

	logger.Info(ctx, ns.logger, "Mounting volume",
		zap.String("bucket_name", secretMap["bucketName"]),
		zap.String("target_path", targetPath))
	if err = mounterObj.Mount(ctx, "", targetPath); err != nil {
		logger.Error(ctx, ns.logger, "Mount failed", zap.Error(err))
		return nil, err
	}

	logger.Info(ctx, ns.logger, "NodePublishVolume completed successfully",
		zap.String("bucket_name", secretMap["bucketName"]),
		zap.String("target_path", targetPath))
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)

	logger.Info(ctx, ns.logger, "NodeUnpublishVolume started")
	logger.Debug(ctx, ns.logger, "NodeUnpublishVolume request", zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		logger.Error(ctx, ns.logger, "Volume ID missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		logger.Error(ctx, ns.logger, "Target path missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}

	logger.Info(ctx, ns.logger, "Unmounting target path", zap.String("target_path", targetPath))

	attrib, err := ns.Stats.GetPVAttributes(volumeID)
	if err != nil {
		logger.Error(ctx, ns.logger, "Failed to get PV details", zap.String("volume_id", volumeID), zap.Error(err))
		return nil, status.Error(codes.NotFound, fmt.Sprintf("[%s] Failed to get PV details", reqID))
	}

	mounterObj := ns.Mounter.NewMounter(attrib, nil, nil, nil)

	logger.Info(ctx, ns.logger, "Unmounting volume")
	if err = mounterObj.Unmount(ctx, targetPath); err != nil {
		//TODO: Need to handle the case with non existing mount separately - https://github.com/IBM/ibm-object-csi-driver/issues/46
		logger.Error(ctx, ns.logger, "Unmount failed", zap.String("target_path", targetPath), zap.Error(err))
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] %v", reqID, err.Error()))
	}

	logger.Info(ctx, ns.logger, "NodeUnpublishVolume completed successfully", zap.String("target_path", targetPath))
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	reqID := requestid.FromContext(ctx)

	logger.Info(ctx, ns.logger, "NodeGetVolumeStats started")
	logger.Debug(ctx, ns.logger, "NodeGetVolumeStats request", zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		logger.Error(ctx, ns.logger, "Volume ID missing in request")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	volumePath := req.VolumePath
	if volumePath == "" {
		logger.Error(ctx, ns.logger, "Volume path doesn't exist")
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Path Doesn't exist", reqID))
	}

	logger.Debug(ctx, ns.logger, "Getting filesystem stats", zap.String("volume_path", volumePath))
	//  Making direct call to fs library for the sake of simplicity. That way we don't need to initialize VolumeStatsUtils. If there is a need for VolumeStatsUtils to grow bigger then we can use it
	_, capacity, _, inodes, inodesFree, inodesUsed, err := ns.Stats.FSInfo(volumePath)

	if err != nil {
		logger.Error(ctx, ns.logger, "Error getting volume stats",
			zap.String("volume_id", volumeID), zap.Error(err))
		return &csi.NodeGetVolumeStatsResponse{
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: true,
				Message:  fmt.Sprintf("[%s] %v", reqID, err.Error()),
			},
		}, nil
	}

	totalCap, err := ns.Stats.GetTotalCapacityFromPV(volumeID)
	if err != nil {
		logger.Error(ctx, ns.logger, "Error getting total capacity from PV", zap.Error(err))
		return nil, err
	}

	capAsInt64, converted := totalCap.AsInt64()
	if !converted {
		capAsInt64 = capacity
		logger.Warn(ctx, ns.logger, "Could not convert capacity, using filesystem capacity", zap.Int64("capacity", capacity))
	}
	logger.Info(ctx, ns.logger, "Total capacity of volume", zap.Int64("capacity", capAsInt64))

	capUsed, err := ns.Stats.GetBucketUsage(volumeID)
	if err != nil {
		logger.Error(ctx, ns.logger, "Error getting bucket usage", zap.Error(err))
		return nil, err
	}

	// Since `capAvailable` can be negative and K8s will roundoff from int64 to uint64 resulting in misleading value
	// capAvailable := capAsInt64 - capUsed

	resp := &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				// Available: capAvailable,
				Total: capAsInt64,
				Used:  capUsed,
				Unit:  csi.VolumeUsage_BYTES,
			},
			{
				Available: inodesFree,
				Total:     inodes,
				Used:      inodesUsed,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
	}

	logger.Info(ctx, ns.logger, "NodeGetVolumeStats completed",
		zap.Int64("total_bytes", capAsInt64),
		zap.Int64("used_bytes", capUsed),
		zap.Int64("total_inodes", inodes),
		zap.Int64("used_inodes", inodesUsed))
	return resp, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, _ *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)

	logger.Info(ctx, ns.logger, "NodeExpandVolume not implemented")
	return &csi.NodeExpandVolumeResponse{}, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] NodeExpandVolume is not implemented", reqID))
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	logger.Info(ctx, ns.logger, "NodeGetCapabilities started")
	logger.Debug(ctx, ns.logger, "NodeGetCapabilities request", zap.Any("request", req))

	var caps []*csi.NodeServiceCapability
	for _, cap := range nodeServerCapabilities {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}

	logger.Info(ctx, ns.logger, "NodeGetCapabilities completed", zap.Int("capability_count", len(caps)))
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	logger.Info(ctx, ns.logger, "NodeGetInfo started")
	logger.Debug(ctx, ns.logger, "NodeGetInfo request", zap.Any("request", req))

	topology := &csi.Topology{
		Segments: map[string]string{
			constants.NodeRegionLabel: ns.Region,
			constants.NodeZoneLabel:   ns.Zone,
		},
	}
	resp := &csi.NodeGetInfoResponse{
		NodeId:             ns.NodeID,
		MaxVolumesPerNode:  ns.MaxVolumesPerNode,
		AccessibleTopology: topology,
	}

	logger.Info(ctx, ns.logger, "NodeGetInfo completed",
		zap.String("node_id", ns.NodeID),
		zap.Int64("max_volumes_per_node", ns.MaxVolumesPerNode),
		zap.String("region", ns.Region),
		zap.String("zone", ns.Zone))
	return resp, nil
}
