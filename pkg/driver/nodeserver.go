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
	"github.com/IBM/satellite-object-storage-plugin/pkg/mounter"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	"os"
	"sync"
)

type nodeServer struct {
	mux sync.Mutex
	*s3Driver
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

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	var (
		val       string
		check     bool
		accessKey string
		secretKey string
	)

	klog.Infof("CSINodeServer-NodePublishVolume...| Request %v", *req)

	ns.mux.Lock()
	defer ns.mux.Unlock()

	volumeID := req.GetVolumeId()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	stagingTargetPath := req.GetStagingTargetPath()

	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging Target path missing in request")
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

	klog.Infof("target %v\ndevice %v\nreadonly %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
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
		attrib["bucket-name"], attrib["obj-path"],
		attrib["cos-endpoint"], attrib["regn-class"],
		fmt.Sprintf("%s:%s", accessKey, secretKey)); err != nil {
		return nil, err
	}

	klog.Info("-NodePublishVolume-: Mount")

	if err = mounterObj.Mount("", targetPath); err != nil {
		return nil, err
	}

	klog.Infof("s3: bucket %s successfuly mounted to %s", attrib["bucket-name"], targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof("CSINodeServer-NodeUnpublishVolume...| Request: %v", *req)
	ns.mux.Lock()
	defer ns.mux.Unlock()

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
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("Successfully unmounted  target path %s", targetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.Infof("CSINodeServer-NodeStageVolume... | Request %v", *req)

	ns.mux.Lock()
	defer ns.mux.Unlock()

	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Volume Capability must be provided")
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(2).Infof("CSINodeServer-NodeUnstageVolume ... | Request %v", *req)
	ns.mux.Lock()
	defer ns.mux.Unlock()

	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	klog.Infof("Unmounting staging target path %s", stagingTargetPath)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodeGetCapabilities returns the supported capabilities of the node server
func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// currently there is a single NodeServer capability according to the spec
	klog.V(4).Infof("NodeGetCapabilities: called with args %+v", *req)
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

func (d *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("NodeGetInfo: called with args %+v", *req)
	return &csi.NodeGetInfoResponse{}, status.Error(codes.Unimplemented, "NodeGetInfo is not implemented")

}

func (d *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(4).Infof("NodeGetVolumeStats: called with args %+v", *req)
	return &csi.NodeGetVolumeStatsResponse{}, status.Error(codes.Unimplemented, "NodeGetVolumeStats is not implemented")

}
