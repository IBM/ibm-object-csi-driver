/**
 * Copyright 2021 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package driver

import (
	"fmt"
	"os"

	"github.com/IBM/satellite-object-storage-plugin/pkg/mounter"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/fs"
	mount "k8s.io/mount-utils"
)

const (
	DefaultVolumesPerNode = 4
)

// Implements Node Server csi.NodeServer
type nodeServer struct {
	*s3Driver
	Stats  statsUtils
	NodeID string
}

type statsUtils interface {
	FSInfo(path string) (int64, int64, int64, int64, int64, int64, error)
	IsBlockDevice(devicePath string) (bool, error)
	DeviceInfo(devicePath string) (int64, error)
	IsDevicePathNotExist(devicePath string) bool
}

type VolumeStatsUtils struct {
}

var (
	mounterObj      mounter.Mounter
	newmounter      = mounter.NewMounter
	fuseunmount     = mounter.FuseUnmount
	checkMountpoint = checkMount
	// nodeCaps represents the capability of node service.
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
	}
)

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(2).Infof("CSINodeServer-NodeStageVolume: Request %v", *req)

	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(2).Infof("CSINodeServer-NodeUnstageVolume: Request %v", *req)

	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	var (
		val       string
		check     bool
		accessKey string
		secretKey string
	)
	klog.V(2).Infof("CSINodeServer-NodePublishVolume: Request %v", *req)

	volumeID := req.GetVolumeId()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}

	notMnt, err := checkMountpoint(targetPath)
	if err != nil {
		klog.Errorf("Can not validate target mount point: %s %v", targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
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

	klog.V(2).Infof("target %v\ndevice %v\nreadonly %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
		targetPath, deviceID, readOnly, volumeID, attrib, mountFlags)

	secretMap := req.GetSecrets()
	if val, check = secretMap["access-key"]; check {
		accessKey = val
	}
	if val, check = secretMap["secret-key"]; check {
		secretKey = val
	}

	//TODO: IAM Implementation for above code snippet

	fmt.Println("CreateVolume VolumeContext:\n\t", attrib)
	fmt.Println("CreateVolume Secrets:\n\t", secretMap)

	//func newMounter(mounter string, bucket string, objpath string, endpoint string, region string, keys string) (Mounter, error) {
	if mounterObj, err = newmounter("s3fs",
		secretMap["bucket-name"], secretMap["obj-path"],
		secretMap["cos-endpoint"], secretMap["regn-class"],
		fmt.Sprintf("%s:%s", accessKey, secretKey)); err != nil {
		return nil, err
	}

	klog.Info("-NodePublishVolume-: Mount")

	if err = mounterObj.Mount("", targetPath); err != nil {
		return nil, err
	}

	klog.Infof("s3: bucket %s successfuly mounted to %s", secretMap["bucket-name"], targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(2).Infof("CSINodeServer-NodeUnpublishVolume: Request: %v", *req)

	volumeID := req.GetVolumeId()
	targetPath := req.GetTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	klog.Infof("Unmounting  target path %s", targetPath)

	if err := fuseunmount(targetPath); err != nil {

		//TODO: Need to handle the case with non existing mount separately - https://github.com/IBM/satellite-object-storage-plugin/issues/46
		klog.Infof("UNMOUNT ERROR: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("Successfully unmounted  target path %s", targetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(2).Infof("NodeGetVolumeStats: Request: %+v", *req)

	if req.VolumePath == "" {
		return nil, status.Error(codes.NotFound, "Path Doesn't exist")
	}

	klog.V(2).Info("NodeGetVolumeStats: Start getting Stats")
	//  Making direct call to fs library for the sake of simplicity. That way we don't need to initialize VolumeStatsUtils. If there is a need for VolumeStatsUtils to grow bigger then we can use it
	available, capacity, usage, inodes, inodesFree, inodesUsed, err := fs.Info(req.VolumePath)

	if err != nil {
		return nil, err
	}

	resp := &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: available,
				Total:     capacity,
				Used:      usage,
				Unit:      csi.VolumeUsage_BYTES,
			},
			{
				Available: inodesFree,
				Total:     inodes,
				Used:      inodesUsed,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
	}

	return resp, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, status.Error(codes.Unimplemented, "NodeExpandVolume is not implemented")
}

// NodeGetCapabilities returns the supported capabilities of the node server
func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// currently there is a single NodeServer capability according to the spec
	klog.V(2).Infof("NodeGetCapabilities: Request: %+v", *req)
	var caps []*csi.NodeServiceCapability
	for _, cap := range nodeCaps {
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

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(3).Infof("NodeGetInfo: called with args %+v", *req)

	resp := &csi.NodeGetInfoResponse{
		NodeId:            ns.NodeID,
		MaxVolumesPerNode: DefaultVolumesPerNode,
	}
	return resp, nil

}

func (su *VolumeStatsUtils) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	return fs.Info(path)
}
