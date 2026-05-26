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
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodeStageVolume started", reqID))
	log.Debug(fmt.Sprintf("[%s] NodeStageVolume request", reqID), zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		log.Error(fmt.Sprintf("[%s] Volume ID missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		log.Error(fmt.Sprintf("[%s] Target path missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}

	log.Info(fmt.Sprintf("[%s] NodeStageVolume completed", reqID),
		zap.String("volume_id", volumeID),
		zap.String("staging_target_path", stagingTargetPath))
	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodeUnstageVolume started", reqID))
	log.Debug(fmt.Sprintf("[%s] NodeUnstageVolume request", reqID), zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		log.Error(fmt.Sprintf("[%s] Volume ID missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		log.Error(fmt.Sprintf("[%s] Target path missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}

	log.Info(fmt.Sprintf("[%s] NodeUnstageVolume completed", reqID),
		zap.String("volume_id", volumeID),
		zap.String("staging_target_path", stagingTargetPath))
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodePublishVolume started", reqID))
	
	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] Error modifying request", reqID), zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Error in modifying requests %v", reqID, err))
	}
	log.Debug(fmt.Sprintf("[%s] NodePublishVolume request", reqID), zap.Any("request", modifiedRequest.(*csi.NodePublishVolumeRequest)))

	volumeMountGroup := req.GetVolumeCapability().GetMount().GetVolumeMountGroup()
	log.Debug(fmt.Sprintf("[%s] Volume mount group", reqID), zap.String("volume_mount_group", volumeMountGroup))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		log.Error(fmt.Sprintf("[%s] Volume ID missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		log.Error(fmt.Sprintf("[%s] Target path missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}

	if req.GetVolumeCapability() == nil {
		log.Error(fmt.Sprintf("[%s] Volume capability missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume capability missing in request", reqID))
	}

	log.Info(fmt.Sprintf("[%s] Checking mount point", reqID), zap.String("target_path", targetPath))
	err = ns.Stats.CheckMount(targetPath)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] Cannot validate target mount point", reqID),
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
	log.Debug(fmt.Sprintf("[%s] NodePublishVolume parameters", reqID),
		zap.String("target_path", targetPath),
		zap.String("device_id", deviceID),
		zap.Bool("readonly", readOnly),
		zap.String("volume_id", volumeID),
		zap.Any("attributes", attrib),
		zap.Strings("mount_flags", mountFlags))

	secretMap := req.GetSecrets()
	log.Info(fmt.Sprintf("[%s] Secrets received", reqID), zap.Int("secret_count", len(secretMap)))
	
	secretMapCopy := make(map[string]string)
	for k, v := range secretMap {
		if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
			secretMapCopy[k] = "xxxxxxx"
			continue
		}
		secretMapCopy[k] = v
	}
	log.Debug(fmt.Sprintf("[%s] Secret map (sanitized)", reqID), zap.Any("secret_map", secretMapCopy))
	
	if volumeMountGroup != "" {
		secretMap["gid"] = volumeMountGroup
		log.Debug(fmt.Sprintf("[%s] Added volume mount group to secrets", reqID), zap.String("gid", volumeMountGroup))
	}

	if len(secretMap["cosEndpoint"]) == 0 {
		secretMap["cosEndpoint"] = attrib["cosEndpoint"]
		log.Debug(fmt.Sprintf("[%s] Using cosEndpoint from attributes", reqID), zap.String("cos_endpoint", secretMap["cosEndpoint"]))
	}

	if len(secretMap["locationConstraint"]) == 0 {
		secretMap["locationConstraint"] = attrib["locationConstraint"]
		log.Debug(fmt.Sprintf("[%s] Using locationConstraint from attributes", reqID), zap.String("location_constraint", secretMap["locationConstraint"]))
	}

	if len(secretMap["cosEndpoint"]) == 0 {
		log.Error(fmt.Sprintf("[%s] S3 Service endpoint not provided", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] S3 Service endpoint not provided", reqID))
	}

	if len(secretMap["iamEndpoint"]) == 0 {
		secretMap["iamEndpoint"] = ns.iamEndpoint
		log.Debug(fmt.Sprintf("[%s] Using default IAM endpoint", reqID), zap.String("iam_endpoint", ns.iamEndpoint))
	}

	// If bucket name wasn't provided by user, we use temp bucket created for volume.
	if secretMap["bucketName"] == "" {
		log.Info(fmt.Sprintf("[%s] Bucket name not provided, fetching from PV", reqID))
		tempBucketName, err := ns.Stats.GetBucketNameFromPV(volumeID)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Unable to fetch PV", reqID), zap.String("volume_id", volumeID), zap.Error(err))
			return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] %v", reqID, err.Error()))
		}

		if tempBucketName == "" {
			log.Error(fmt.Sprintf("[%s] Unable to fetch bucket name from PV", reqID))
			return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] unable to fetch bucket name from pv", reqID))
		}

		secretMap["bucketName"] = tempBucketName
		log.Info(fmt.Sprintf("[%s] Using bucket from PV", reqID), zap.String("bucket_name", tempBucketName))
	}

	var defaultParamsMap = map[string]string{
		constants.CipherSuitesKey: ns.TLSCipherSuite,
	}

	log.Info(fmt.Sprintf("[%s] Creating mounter object", reqID))
	mounterObj := ns.Mounter.NewMounter(attrib, secretMap, mountFlags, defaultParamsMap)

	log.Info(fmt.Sprintf("[%s] Mounting volume", reqID),
		zap.String("bucket_name", secretMap["bucketName"]),
		zap.String("target_path", targetPath))
	if err = mounterObj.Mount(ctx, "", targetPath); err != nil {
		log.Error(fmt.Sprintf("[%s] Mount failed", reqID), zap.Error(err))
		return nil, err
	}

	log.Info(fmt.Sprintf("[%s] NodePublishVolume completed successfully", reqID),
		zap.String("bucket_name", secretMap["bucketName"]),
		zap.String("target_path", targetPath))
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodeUnpublishVolume started", reqID))
	log.Debug(fmt.Sprintf("[%s] NodeUnpublishVolume request", reqID), zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		log.Error(fmt.Sprintf("[%s] Volume ID missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		log.Error(fmt.Sprintf("[%s] Target path missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Target path missing in request", reqID))
	}
	
	log.Info(fmt.Sprintf("[%s] Unmounting target path", reqID), zap.String("target_path", targetPath))

	attrib, err := ns.Stats.GetPVAttributes(volumeID)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] Failed to get PV details", reqID), zap.String("volume_id", volumeID), zap.Error(err))
		return nil, status.Error(codes.NotFound, fmt.Sprintf("[%s] Failed to get PV details", reqID))
	}

	mounterObj := ns.Mounter.NewMounter(attrib, nil, nil, nil)

	log.Info(fmt.Sprintf("[%s] Unmounting volume", reqID))
	if err = mounterObj.Unmount(ctx, targetPath); err != nil {
		//TODO: Need to handle the case with non existing mount separately - https://github.com/IBM/ibm-object-csi-driver/issues/46
		log.Error(fmt.Sprintf("[%s] Unmount failed", reqID), zap.String("target_path", targetPath), zap.Error(err))
		return nil, status.Error(codes.Internal, fmt.Sprintf("[%s] %v", reqID, err.Error()))
	}

	log.Info(fmt.Sprintf("[%s] NodeUnpublishVolume completed successfully", reqID), zap.String("target_path", targetPath))
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodeGetVolumeStats started", reqID))
	log.Debug(fmt.Sprintf("[%s] NodeGetVolumeStats request", reqID), zap.Any("request", req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		log.Error(fmt.Sprintf("[%s] Volume ID missing in request", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Volume ID missing in request", reqID))
	}

	volumePath := req.VolumePath
	if volumePath == "" {
		log.Error(fmt.Sprintf("[%s] Volume path doesn't exist", reqID))
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("[%s] Path Doesn't exist", reqID))
	}

	log.Debug(fmt.Sprintf("[%s] Getting filesystem stats", reqID), zap.String("volume_path", volumePath))
	//  Making direct call to fs library for the sake of simplicity. That way we don't need to initialize VolumeStatsUtils. If there is a need for VolumeStatsUtils to grow bigger then we can use it
	_, capacity, _, inodes, inodesFree, inodesUsed, err := ns.Stats.FSInfo(volumePath)

	if err != nil {
		log.Error(fmt.Sprintf("[%s] Error getting volume stats", reqID),
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
		log.Error(fmt.Sprintf("[%s] Error getting total capacity from PV", reqID), zap.Error(err))
		return nil, err
	}

	capAsInt64, converted := totalCap.AsInt64()
	if !converted {
		capAsInt64 = capacity
		log.Warn(fmt.Sprintf("[%s] Could not convert capacity, using filesystem capacity", reqID), zap.Int64("capacity", capacity))
	}
	log.Info(fmt.Sprintf("[%s] Total capacity of volume", reqID), zap.Int64("capacity", capAsInt64))

	capUsed, err := ns.Stats.GetBucketUsage(volumeID)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] Error getting bucket usage", reqID), zap.Error(err))
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

	log.Info(fmt.Sprintf("[%s] NodeGetVolumeStats completed", reqID),
		zap.Int64("total_bytes", capAsInt64),
		zap.Int64("used_bytes", capUsed),
		zap.Int64("total_inodes", inodes),
		zap.Int64("used_inodes", inodesUsed))
	return resp, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, _ *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodeExpandVolume not implemented", reqID))
	return &csi.NodeExpandVolumeResponse{}, status.Error(codes.Unimplemented, fmt.Sprintf("[%s] NodeExpandVolume is not implemented", reqID))
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodeGetCapabilities started", reqID))
	log.Debug(fmt.Sprintf("[%s] NodeGetCapabilities request", reqID), zap.Any("request", req))
	
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
	
	log.Info(fmt.Sprintf("[%s] NodeGetCapabilities completed", reqID), zap.Int("capability_count", len(caps)))
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := ns.logger.With(zap.String("request_id", reqID))
	
	log.Info(fmt.Sprintf("[%s] NodeGetInfo started", reqID))
	log.Debug(fmt.Sprintf("[%s] NodeGetInfo request", reqID), zap.Any("request", req))

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
	
	log.Info(fmt.Sprintf("[%s] NodeGetInfo completed", reqID),
		zap.String("node_id", ns.NodeID),
		zap.Int64("max_volumes_per_node", ns.MaxVolumesPerNode),
		zap.String("region", ns.Region),
		zap.String("zone", ns.Zone))
	return resp, nil
}
