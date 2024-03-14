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
	"os/exec"
	"strings"

	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/fs"
)

const (
	DefaultVolumesPerNode = 4
)

// Implements Node Server csi.NodeServer
type nodeServer struct {
	*S3Driver
	Stats   statsUtils
	NodeID  string
	Mounter mounter.NewMounterFactory
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
	modifiedRequest, err := ReplaceAndReturnCopy(req, "xxx", "yyy")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Error in modifying requests %v", err))
	}
	klog.V(2).Infof("CSINodeServer-NodePublishVolume: Request %v", modifiedRequest.(*csi.NodePublishVolumeRequest))

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

	klog.V(2).Infof("-NodePublishVolume-: targetPath: %v\ndeviceID: %v\nreadonly: %v\nvolumeId: %v\nattributes: %v\nmountFlags: %v\n",
		targetPath, deviceID, readOnly, volumeID, attrib, mountFlags)

	secretMap := req.GetSecrets()
	secretMapCopy := make(map[string]string)
	for k, v := range secretMap {
		if k == "accessKey" || k == "secretKey" || k == "apiKey" {
			secretMapCopy[k] = "xxxxxxx"
			continue
		}
		secretMapCopy[k] = v
	}
	klog.V(2).Infof("-NodePublishVolume-: secretMap: %v", secretMapCopy)

	// If bucket name wasn't provided by user, we use temp bucket created for volume.
	if secretMap["bucketName"] == "" {
		config, err := rest.InClusterConfig()
		if err != nil {
			klog.Errorf("Unable to get cluster config %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			klog.Errorf("Unable to create client %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		pv, err := clientset.CoreV1().PersistentVolumes().Get(context.Background(), volumeID, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Unable to fetch pv %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		tempBucketName := pv.Spec.CSI.VolumeAttributes["bucketName"]

		if tempBucketName == "" {
			klog.Errorf("Unable to fetch bucket name from pv %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		secretMap["bucketName"] = tempBucketName
	}

	if mounterObj, err = ns.Mounter.NewMounter(attrib, secretMap, mountFlags); err != nil {
		return nil, err
	}

	klog.Info("-NodePublishVolume-: Mount")

	if err = mounterObj.Mount("", targetPath); err != nil {
		klog.Info("-Mount-: Error %v", err)
		return nil, err
	}

	klog.Infof("s3: bucket %s successfully mounted to %s", secretMap["bucket-name"], targetPath)
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

		//TODO: Need to handle the case with non existing mount separately - https://github.com/IBM/ibm-object-csi-driver/issues/46
		klog.Infof("UNMOUNT ERROR: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("Successfully unmounted  target path %s", targetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(2).Infof("NodeGetVolumeStats: Request: %+v", *req)

	volumeID := req.GetVolumeId()
	if req.VolumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "Path Doesn't exist")
	}
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	klog.V(2).Info("NodeGetVolumeStats: Start getting Stats")
	//  Making direct call to fs library for the sake of simplicity. That way we don't need to initialize VolumeStatsUtils. If there is a need for VolumeStatsUtils to grow bigger then we can use it
	available, capacity, usage, inodes, inodesFree, inodesUsed, err := fs.Info(req.VolumePath)

	fmt.Println("ERROR:", err)
	fmt.Println()
	fmt.Println("NOERR:", available, capacity, usage, inodes, inodesFree, inodesUsed)

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
	out, err := exec.Command("mountpoint", targetPath).CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	notMnt := true
	if err != nil {
		klog.V(3).Infof("Output: Output string error %+v", outStr)
		if strings.HasSuffix(outStr, "No such file or directory") {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				klog.V(2).Infof("checkMount: Error: %+v", err)
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
	top := &csi.Topology{}
	resp := &csi.NodeGetInfoResponse{
		NodeId:             ns.NodeID,
		MaxVolumesPerNode:  DefaultVolumesPerNode,
		AccessibleTopology: top,
	}
	fmt.Println(resp)
	return resp, nil

}

func (su *VolumeStatsUtils) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	return fs.Info(path)
}
