/**
 * Copyright 2024 IBM Corp.
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
	"bytes"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const defaultMode = "controller"

// GetTestLogger ...
func GetTestLogger(t *testing.T) (logger *zap.Logger, teardown func()) {

	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	buf := &bytes.Buffer{}

	logger, _ = config.Build()

	teardown = func() {
		_ = logger.Sync()
		if t.Failed() {
			t.Log(buf)
		}
	}

	return logger, teardown
}

func TestAddVolumeCapabilityAccessModes(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	driver := &S3Driver{
		logger: logger,
	}
	defer teardown()
	err := driver.AddVolumeCapabilityAccessModes(volumeCapabilities)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(driver.vcap) != len(volumeCapabilities) {
		t.Errorf("expected %d volume capabilities, got %d", len(volumeCapabilities), len(driver.vcap))
	}
}

func TestAddControllerServiceCapabilities(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	driver := &S3Driver{
		logger: logger,
	}
	defer teardown()
	err := driver.AddControllerServiceCapabilities(controllerCapabilities)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(driver.cscap) != len(controllerCapabilities) {
		t.Errorf("expected %d controller capabilities, got %d", len(controllerCapabilities), len(driver.cscap))
	}
}

func TestAddNodeServiceCapabilities(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	driver := &S3Driver{
		logger: logger,
	}
	defer teardown()
	err := driver.AddNodeServiceCapabilities(nodeServerCapabilities)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(driver.nscap) != len(nodeServerCapabilities) {
		t.Errorf("expected %d node capabilities, got %d", len(nodeServerCapabilities), len(driver.nscap))
	}
}

func TestNewS3CosDriver(t *testing.T) {
	vendorVersion := "test-vendor-version-1.1.2"
	driverName := "mydriver"

	endpoint := "test-endpoint"
	nodeID := "test-nodeID"

	fakeCosSession := &s3client.FakeCOSSessionFactory{}
	fakeMountObj := &mounter.FakeMounterFactory{}

	logger, teardown := GetTestLogger(t)
	defer teardown()

	// Setup the CSI driver
	driver, err := Setups3Driver(defaultMode, driverName, vendorVersion, logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, driver)

	statsUtil := utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{})
	mounterUtil := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{})

	csiDriver, err := driver.NewS3CosDriver(nodeID, endpoint, fakeCosSession, fakeMountObj, statsUtil, mounterUtil)
	assert.NoError(t, err)
	assert.NotEmpty(t, csiDriver)
}

func TestNewS3CosDriver_mode_node(t *testing.T) {
	vendorVersion := "test-vendor-version-1.1.2"
	driverName := "mydriver"

	endpoint := "test-endpoint"
	nodeID := "test-nodeID"

	fakeCosSession := &s3client.FakeCOSSessionFactory{}
	fakeMountObj := &mounter.FakeMounterFactory{}

	logger, teardown := GetTestLogger(t)
	defer teardown()

	// Setup the CSI driver
	driver, err := Setups3Driver("node", driverName, vendorVersion, logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, driver)

	statsUtil := utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{})
	mounterUtil := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{})

	csiDriver, err := driver.NewS3CosDriver(nodeID, endpoint, fakeCosSession, fakeMountObj, statsUtil, mounterUtil)
	assert.NoError(t, err)
	assert.NotEmpty(t, csiDriver)
}

func TestNewS3CosDriver_mode_controller_node(t *testing.T) {
	vendorVersion := "test-vendor-version-1.1.2"
	driverName := "mydriver"

	endpoint := "test-endpoint"
	nodeID := "test-nodeID"

	fakeCosSession := &s3client.FakeCOSSessionFactory{}
	fakeMountObj := &mounter.FakeMounterFactory{}

	logger, teardown := GetTestLogger(t)
	defer teardown()

	// Setup the CSI driver
	driver, err := Setups3Driver("controller-node", driverName, vendorVersion, logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, driver)

	statsUtil := utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{})
	mounterUtil := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{})

	csiDriver, err := driver.NewS3CosDriver(nodeID, endpoint, fakeCosSession, fakeMountObj, statsUtil, mounterUtil)
	assert.NoError(t, err)
	assert.NotEmpty(t, csiDriver)
}

func TestSetups3Driver_Positive(t *testing.T) {
	vendorVersion := "test-vendor"
	driverName := "test-driver"
	logger, teardown := GetTestLogger(t)
	defer teardown()

	csiDriver, err := Setups3Driver(defaultMode, driverName, vendorVersion, logger)
	assert.Nil(t, err)
	assert.NotEmpty(t, csiDriver)

	assert.Equal(t, csiDriver.name, driverName)
	assert.Equal(t, csiDriver.version, vendorVersion)
	assert.Equal(t, csiDriver.mode, defaultMode)
}

func TestSetups3Driver_Negative(t *testing.T) {
	vendorVersion := "test-vendor"
	driverName := ""
	logger, teardown := GetTestLogger(t)
	defer teardown()

	_, err := Setups3Driver(defaultMode, driverName, vendorVersion, logger)
	assert.NotNil(t, err)
}
