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
	commonError "github.ibm.com/alchemy-containers/ibm-csi-common/pkg/messages"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type identityServer struct {
	*csicommon.DefaultIdentityServer
	*s3Driver
}

// GetPluginInfo ...
func (csiIdentity *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	ctxLogger, requestID := utils.GetContextLogger(ctx, false)
	ctxLogger.Info("identityServer-GetPluginInfo...", zap.Reflect("Request", *req))

	if csiIdentity.s3Driver == nil {
		return nil, commonError.GetCSIError(ctxLogger, commonError.DriverNotConfigured, requestID, nil)
	}

	return &csi.GetPluginInfoResponse{
		Name:          csiIdentity.s3Driver.name,
		VendorVersion: csiIdentity.s3Driver.vendorVersion,
	}, nil
}

// GetPluginCapabilities ...
func (csiIdentity *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	ctxLogger, _ := utils.GetContextLogger(ctx, false)
	ctxLogger.Info("identityServer-GetPluginCapabilities...", zap.Reflect("Request", *req))

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
	ctxLogger, _ := utils.GetContextLogger(ctx, false)
	ctxLogger.Info("identityServer-Probe...", zap.Reflect("Request", *req))
	return &csi.ProbeResponse{}, nil
}
