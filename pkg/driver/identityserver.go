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
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type identityServer struct {
	*csicommon.DefaultIdentityServer
	*s3Driver
}

// GetPluginInfo ...
func (csiIdentity *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	klog.Infof("identityServer-GetPluginInfo... | Request: %v", *req)
	if csiIdentity.s3Driver == nil {
		return nil, status.Error(codes.InvalidArgument, "Driver not configured")
	}

	return &csi.GetPluginInfoResponse{
		Name:          csiIdentity.s3Driver.name,
		VendorVersion: csiIdentity.s3Driver.vendorVersion,
	}, nil
}

// GetPluginCapabilities ...
func (csiIdentity *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	klog.Infof("identityServer-GetPluginCapabilities...| Request %v", *req)
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

// Probe ...
func (csiIdentity *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	klog.Infof("identityServer-Probe... Request %v", *req)
	return &csi.ProbeResponse{}, nil
}
