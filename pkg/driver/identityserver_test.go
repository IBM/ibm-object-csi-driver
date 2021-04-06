/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

//Package driver ...
package driver

import (
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestGetPluginInfo(t *testing.T) {
	vendorVersion := "test-vendor-version-1.1.2"
	driver := "mydriver"

	icDriver := inits3Driver(t)
	if icDriver == nil {
		t.Fatalf("Failed to setup IBM CSI Driver")
	}
	// Get the plugin response by using driver
	resp, err := icDriver.ids.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo returned unexpected error: %v", err)
	}

	if resp.GetName() != driver {
		t.Fatalf("Response name expected: %v, got: %v", driver, resp.GetName())
	}

	respVer := resp.GetVendorVersion()
	if respVer != vendorVersion {
		t.Fatalf("Vendor version expected: %v, got: %v", vendorVersion, respVer)
	}

	// set driver as nil
	icDriver.ids.s3Driver = nil
	resp, err = icDriver.ids.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
	assert.NotNil(t, err)
	assert.Nil(t, resp)
}

func TestGetPluginCapabilities(t *testing.T) {
	icDriver := inits3Driver(t)
	if icDriver == nil {
		t.Fatalf("Failed to setup IBM CSI Driver")
	}

	resp, err := icDriver.ids.GetPluginCapabilities(context.Background(), &csi.GetPluginCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("GetPluginCapabilities returned unexpected error: %v", err)
	}

	for _, capability := range resp.GetCapabilities() {
		switch capability.GetService().GetType() {
		case csi.PluginCapability_Service_CONTROLLER_SERVICE:
		case csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS:
		default:
			t.Fatalf("Unknown capability: %v", capability.GetService().GetType())
		}
	}
}

func TestProbe(t *testing.T) {
	icDriver := inits3Driver(t)
	if icDriver == nil {
		t.Fatalf("Failed to setup IBM CSI Driver")
	}

	_, err := icDriver.ids.Probe(context.Background(), &csi.ProbeRequest{})
	if err != nil {
		t.Fatalf("Probe returned unexpected error: %v", err)
	}
}
