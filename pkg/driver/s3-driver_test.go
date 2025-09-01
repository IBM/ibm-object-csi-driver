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
	"errors"
	"strconv"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
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

func TestNewNodeServer(t *testing.T) {
	vendorVersion := "test-vendor-version-1.1.2"
	driverName := "test-csi-driver"

	nodeID := "test-nodeID"
	testRegion := "test-region"
	testZone := "test-zone"

	testCases := []struct {
		testCaseName string
		envVars      map[string]string
		statsUtils   utils.StatsUtils
		verifyResult func(*testing.T, *nodeServer, error)
		expectedErr  error
	}{
		{
			testCaseName: "Positive: success",
			envVars: map[string]string{
				constants.KubeNodeName:         nodeID,
				constants.MaxVolumesPerNodeEnv: "10",
			},
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetNodeServerDataFn: func(nodeName string) (*utils.NodeServerData, error) {
					return &utils.NodeServerData{
						Region: testRegion,
						Zone:   testZone,
					}, nil
				},
			}),
			verifyResult: func(t *testing.T, ns *nodeServer, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, ns)
				assert.Equal(t, ns.MaxVolumesPerNode, int64(10))
				assert.Equal(t, ns.Region, testRegion)
				assert.Equal(t, ns.Zone, testZone)
				assert.Equal(t, ns.NodeID, nodeID)
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Negative: Failed to get KUBE_NODE_NAME env variable",
			envVars: map[string]string{
				constants.KubeNodeName:         "",
				constants.MaxVolumesPerNodeEnv: "",
			},
			verifyResult: func(t *testing.T, ns *nodeServer, err error) {
				assert.Nil(t, ns)
			},
			expectedErr: errors.New("KUBE_NODE_NAME env variable not set"),
		},
		{
			testCaseName: "Negative: Failed to get region and zone",
			envVars: map[string]string{
				constants.KubeNodeName:         nodeID,
				constants.MaxVolumesPerNodeEnv: "",
			},
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetNodeServerDataFn: func(nodeName string) (*utils.NodeServerData, error) {
					return nil, errors.New("unable to load in-cluster configuration")
				},
			}),
			verifyResult: func(t *testing.T, ns *nodeServer, err error) {
				assert.Nil(t, ns)
			},
			expectedErr: errors.New("unable to load in-cluster configuration"),
		},
		{
			testCaseName: "Negative: invalid value of maxVolumesPerNode",
			envVars: map[string]string{
				constants.KubeNodeName:         nodeID,
				constants.MaxVolumesPerNodeEnv: "invalid",
			},
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetNodeServerDataFn: func(nodeName string) (*utils.NodeServerData, error) {
					return &utils.NodeServerData{
						Region: testRegion,
						Zone:   testZone,
					}, nil
				},
			}),
			verifyResult: func(t *testing.T, ns *nodeServer, err error) {
				assert.Nil(t, ns)
			},
			expectedErr: errors.New("invalid syntax"),
		},
		{
			testCaseName: "Positive: maxVolumesPerNode not set",
			envVars: map[string]string{
				constants.KubeNodeName:         nodeID,
				constants.MaxVolumesPerNodeEnv: "",
			},
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetNodeServerDataFn: func(nodeName string) (*utils.NodeServerData, error) {
					return &utils.NodeServerData{
						Region: testRegion,
						Zone:   testZone,
					}, nil
				},
			}),
			verifyResult: func(t *testing.T, ns *nodeServer, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, ns)
				assert.Equal(t, ns.MaxVolumesPerNode, int64(0))
				assert.Equal(t, ns.Region, testRegion)
				assert.Equal(t, ns.Zone, testZone)
				assert.Equal(t, ns.NodeID, nodeID)
			},
			expectedErr: nil,
		},
	}

	logger, teardown := GetTestLogger(t)
	defer teardown()

	// Setup the CSI driver
	driver, err := Setups3Driver("node", driverName, vendorVersion, logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, driver)

	fakeMountObj := &mounter.FakeMounterFactory{}
	mounterUtil := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{})

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		for k, v := range tc.envVars {
			t.Setenv(k, v)
		}

		actualResp, actualErr := newNodeServer(driver, tc.statsUtils, nodeID, fakeMountObj, mounterUtil)

		if tc.expectedErr != nil {
			assert.Error(t, actualErr)
			assert.Contains(t, actualErr.Error(), tc.expectedErr.Error())
		} else {
			assert.NoError(t, actualErr)
		}

		if tc.verifyResult != nil {
			tc.verifyResult(t, actualResp, actualErr)
		}
	}
}

func TestNewS3CosDriver(t *testing.T) {
	vendorVersion := "test-vendor-version-1.1.2"
	driverName := "test-csi-driver"

	endpoint := "test-endpoint"
	nodeID := "test-nodeID"
	testRegion := "test-region"
	testZone := "test-zone"

	envVars := map[string]string{
		constants.KubeNodeName:         testNodeID,
		constants.MaxVolumesPerNodeEnv: strconv.Itoa(constants.DefaultVolumesPerNode),
	}

	testCases := []struct {
		testCaseName string
		mode         string
		statsUtils   utils.StatsUtils
		verifyResult func(*testing.T, *S3Driver, error)
		expectedErr  error
	}{
		{
			testCaseName: "Positive: controller mode",
			mode:         "controller",
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetEndpointsFn: func() (string, string, error) {
					return constants.PublicIAMEndpoint, "", nil
				},
			}),
			verifyResult: func(t *testing.T, driver *S3Driver, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, driver.cs)
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Positive: node mode",
			mode:         "node",
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetEndpointsFn: func() (string, string, error) {
					return constants.PublicIAMEndpoint, "", nil
				},
				GetNodeServerDataFn: func(nodeName string) (*utils.NodeServerData, error) {
					return &utils.NodeServerData{
						Region: testRegion,
						Zone:   testZone,
					}, nil
				},
			}),
			verifyResult: func(t *testing.T, driver *S3Driver, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, driver.ns)
				assert.Equal(t, driver.ns.Region, testRegion)
				assert.Equal(t, driver.ns.Zone, testZone)
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Positive: controller and node mode",
			mode:         "controller-node",
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetEndpointsFn: func() (string, string, error) {
					return constants.PublicIAMEndpoint, "", nil
				},
				GetNodeServerDataFn: func(nodeName string) (*utils.NodeServerData, error) {
					return &utils.NodeServerData{
						Region: testRegion,
						Zone:   testZone,
					}, nil
				},
			}),
			verifyResult: func(t *testing.T, driver *S3Driver, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, driver.cs)
				assert.NotEmpty(t, driver.ns)
				assert.Equal(t, driver.ns.Region, testRegion)
				assert.Equal(t, driver.ns.Zone, testZone)
			},
			expectedErr: nil,
		},
		{
			testCaseName: "Negative: Failed to GetEndpoints",
			mode:         "controller-node",
			statsUtils: utils.NewFakeStatsUtilsImpl(utils.FakeStatsUtilsFuncStruct{
				GetEndpointsFn: func() (string, string, error) {
					return "", "", errors.New("failed")
				},
			}),
			verifyResult: nil,
			expectedErr:  errors.New("failed"),
		},
	}

	fakeCosSession := &s3client.FakeCOSSessionFactory{}
	fakeMountObj := &mounter.FakeMounterFactory{}

	logger, teardown := GetTestLogger(t)
	defer teardown()

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	mounterUtil := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{})

	for _, tc := range testCases {
		t.Log("Testcase being executed", zap.String("testcase", tc.testCaseName))

		// Setup the CSI driver
		driver, err := Setups3Driver(tc.mode, driverName, vendorVersion, logger)
		assert.NoError(t, err)
		assert.NotEmpty(t, driver)

		actualResp, actualErr := driver.NewS3CosDriver(nodeID, endpoint, fakeCosSession, fakeMountObj, tc.statsUtils, mounterUtil)

		if tc.expectedErr != nil {
			assert.Error(t, actualErr)
			assert.Contains(t, actualErr.Error(), tc.expectedErr.Error())
		} else {
			assert.NoError(t, actualErr)
		}

		if tc.verifyResult != nil {
			tc.verifyResult(t, actualResp, actualErr)
		}
	}
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
