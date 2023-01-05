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

// GetPluginInfo ...
func (csiIdentity *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	klog.V(3).Infof("identityServer-GetPluginInfo: Request: %v", *req)
	if csiIdentity.S3Driver == nil {
		return nil, status.Error(codes.InvalidArgument, "Driver not configured")
	}

	return &csi.GetPluginInfoResponse{
		Name:          csiIdentity.S3Driver.name,
		VendorVersion: csiIdentity.S3Driver.version,
	}, nil
}

// GetPluginCapabilities ...
func (csiIdentity *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
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

// Probe ...
func (csiIdentity *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	klog.V(3).Infof("identityServer-Probe: Request %v", *req)
	return &csi.ProbeResponse{}, nil
}
