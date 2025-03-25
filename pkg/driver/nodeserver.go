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
	"os"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// Implements Node Server csi.NodeServer
type nodeServer struct {
	*S3Driver
	Stats        utils.StatsUtils
	NodeID       string
	Mounter      mounter.NewMounterFactory
	MounterUtils mounterUtils.MounterUtils
}

func (ns *nodeServer) NodeStageVolume(_ context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(2).Infof("CSINodeServer-NodeStageVolume: Request %v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(_ context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(2).Infof("CSINodeServer-NodeUnstageVolume: Request %v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	modifiedRequest, err := utils.ReplaceAndReturnCopy(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in modifying requests %v", err))
	}
	klog.V(2).Infof("CSINodeServer-NodePublishVolume: Request %v", modifiedRequest.(*csi.NodePublishVolumeRequest))

	volumeMountGroup := req.GetVolumeCapability().GetMount().GetVolumeMountGroup()
	klog.V(2).Infof("CSINodeServer-NodePublishVolume-: volumeMountGroup: %v", volumeMountGroup)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}

	err = ns.Stats.CheckMount(targetPath)
	if err != nil {
		klog.Errorf("Can not validate target mount point: %s %v", targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	deviceID := ""
	if req.GetPublishContext() != nil {
		deviceID = req.GetPublishContext()[deviceID]
	}

	readOnly := req.GetReadonly()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()
	klog.V(2).Infof("-NodePublishVolume-: targetPath: %v\ndeviceID: %v\nreadonly: %v\nvolumeId: %v\nattributes: %v\nmountFlags: %v\n",
		targetPath, deviceID, readOnly, volumeID, attrib, mountFlags)

	secretMap := req.GetSecrets()
	klog.V(2).Infof("-NodePublishVolume-: length of req.GetSecrets() length: %v", len(req.GetSecrets()))
	secretMapCopy := make(map[string]string)
	for k, v := range secretMap {
		if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
			secretMapCopy[k] = "xxxxxxx"
			continue
		}
		secretMapCopy[k] = v
	}
	klog.V(2).Infof("-NodePublishVolume-: secretMap: %v", secretMapCopy)
	if volumeMountGroup != "" {
		secretMap["gid"] = volumeMountGroup
	}

	if len(secretMap["cosEndpoint"]) == 0 {
		secretMap["cosEndpoint"] = attrib["cosEndpoint"]
	}

	if len(secretMap["locationConstraint"]) == 0 {
		secretMap["locationConstraint"] = attrib["locationConstraint"]
	}

	if len(secretMap["cosEndpoint"]) == 0 {
		return nil, status.Error(codes.InvalidArgument, "S3 Service endpoint not provided")
	}

	// If bucket name wasn't provided by user, we use temp bucket created for volume.
	if secretMap["bucketName"] == "" {
		tempBucketName, err := ns.Stats.GetBucketNameFromPV(volumeID)
		if err != nil {
			klog.Errorf("Unable to fetch pv %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		if tempBucketName == "" {
			klog.Errorf("Unable to fetch bucket name from pv")
			return nil, status.Error(codes.Internal, "unable to fetch bucket name from pv")
		}

		secretMap["bucketName"] = tempBucketName
	}

	mounterObj := ns.Mounter.NewMounter(attrib, secretMap, mountFlags)

	klog.Info("-NodePublishVolume-: Mount")

	if err = mounterObj.Mount("", targetPath, secretMap); err != nil {
		klog.Info("-Mount-: Error: ", err)
		return nil, err
	}

	klog.Infof("s3: bucket %s successfully mounted to %s", secretMap["bucketName"], targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(2).Infof("CSINodeServer-NodeUnpublishVolume: Request: %v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	klog.Infof("Unmounting  target path %s", targetPath)

	if err := ns.MounterUtils.FuseUnmount(targetPath); err != nil {
		//TODO: Need to handle the case with non existing mount separately - https://github.com/IBM/ibm-object-csi-driver/issues/46
		klog.Infof("UNMOUNT ERROR: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("Successfully unmounted  target path %s", targetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(_ context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(2).Infof("NodeGetVolumeStats: Request: %+v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	volumePath := req.VolumePath
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "Path Doesn't exist")
	}

	klog.V(2).Info("NodeGetVolumeStats: Start getting Stats")
	//  Making direct call to fs library for the sake of simplicity. That way we don't need to initialize VolumeStatsUtils. If there is a need for VolumeStatsUtils to grow bigger then we can use it
	_, capacity, _, inodes, inodesFree, inodesUsed, err := ns.Stats.FSInfo(volumePath)

	if err != nil {
		data := map[string]string{"VolumeId": volumeID, "Error": err.Error()}
		klog.Error("NodeGetVolumeStats: error occurred while getting volume stats ", data)
		return &csi.NodeGetVolumeStatsResponse{
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: true,
				Message:  err.Error(),
			},
		}, nil
	}

	totalCap, err := ns.Stats.GetTotalCapacityFromPV(volumeID)
	if err != nil {
		return nil, err
	}

	capAsInt64, converted := totalCap.AsInt64()
	if !converted {
		capAsInt64 = capacity
	}
	klog.Info("NodeGetVolumeStats: Total Capacity of Volume: ", capAsInt64)

	capUsed, err := ns.Stats.GetBucketUsage(volumeID)
	if err != nil {
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

	klog.V(2).Info("NodeGetVolumeStats: Volume Stats ", resp)
	return resp, nil
}

func (ns *nodeServer) NodeExpandVolume(_ context.Context, _ *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, status.Error(codes.Unimplemented, "NodeExpandVolume is not implemented")
}

func (ns *nodeServer) NodeGetCapabilities(_ context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(2).Infof("NodeGetCapabilities: Request: %+v", *req)
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
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (ns *nodeServer) NodeGetInfo(_ context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(3).Infof("NodeGetInfo: called with args %+v", *req)

	nodeName := os.Getenv("KUBE_NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("KUBE_NODE_NAME env variable not set")
	}

	region, zone, err := ns.Stats.GetRegionAndZone(nodeName)
	if err != nil {
		return nil, err
	}

	klog.Infof("NodeGetInfo: Node region %s", region)
	klog.Infof("NodeGetInfo: Node zone %s", zone)

	topology := &csi.Topology{
		Segments: map[string]string{
			constants.NodeRegionLabel: region,
			constants.NodeZoneLabel:   zone,
		},
	}
	resp := &csi.NodeGetInfoResponse{
		NodeId:             ns.NodeID,
		MaxVolumesPerNode:  constants.DefaultVolumesPerNode,
		AccessibleTopology: topology,
	}
	klog.V(2).Info("NodeGetInfo: ", resp)
	return resp, nil
}
