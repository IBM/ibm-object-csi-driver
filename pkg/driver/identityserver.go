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
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// Implements Identity Sever csi.IdentityServe
type identityServer struct {
	*S3Driver
}

func (csiIdentity *identityServer) GetPluginInfo(_ context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	klog.V(3).Infof("identityServer-GetPluginInfo: Request: %v", *req)
	if csiIdentity.S3Driver == nil {
		return nil, status.Error(codes.InvalidArgument, "Driver not configured")
	}

	return &csi.GetPluginInfoResponse{
		Name:          csiIdentity.S3Driver.name,
		VendorVersion: csiIdentity.S3Driver.version,
	}, nil
}

func (csiIdentity *identityServer) GetPluginCapabilities(_ context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	klog.V(3).Infof("identityServer-GetPluginCapabilities: Request %v", *req)
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
			/* TODO Add Volume Expansion {
				 Type: &csi.PluginCapability_Service_{
					 Service: &csi.PluginCapability_Service{
						 Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					 },
				 },
			 }, */
		},
	}, nil
}

func (csiIdentity *identityServer) Probe(_ context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	klog.V(3).Infof("identityServer-Probe: Request %v", *req)
	return &csi.ProbeResponse{}, nil
}
