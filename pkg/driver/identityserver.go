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

	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Implements Identity Sever csi.IdentityServe
type identityServer struct {
	*S3Driver
	csi.UnimplementedIdentityServer
}

func (csiIdentity *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := csiIdentity.logger.With(zap.String("request_id", reqID))
	
	log.Debug("identityServer-GetPluginInfo", zap.Any("request", req))
	if csiIdentity.S3Driver == nil {
		return nil, status.Error(codes.InvalidArgument, "Driver not configured")
	}

	return &csi.GetPluginInfoResponse{
		Name:          csiIdentity.name,
		VendorVersion: csiIdentity.version,
	}, nil
}

func (csiIdentity *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := csiIdentity.logger.With(zap.String("request_id", reqID))
	
	log.Debug("identityServer-GetPluginCapabilities", zap.Any("request", req))
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

func (csiIdentity *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	reqID := requestid.FromContext(ctx)
	log := csiIdentity.logger.With(zap.String("request_id", reqID))
	
	log.Debug("identityServer-Probe", zap.Any("request", req))
	return &csi.ProbeResponse{}, nil
}
