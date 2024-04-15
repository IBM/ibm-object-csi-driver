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
	"bytes"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const defaultMode = "controller"

// GetTestLogger ...
func GetTestLogger(t *testing.T) (logger *zap.Logger, teardown func()) {

	atom := zap.NewAtomicLevel()
	atom.SetLevel(zap.DebugLevel)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	buf := &bytes.Buffer{}

	logger = zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(buf),
			atom,
		),
		zap.AddCaller(),
	)

	teardown = func() {
		_ = logger.Sync()
		if t.Failed() {
			t.Log(buf)
		}
	}
	return
}

func inits3Driver(t *testing.T) *S3Driver {
	vendorVersion := "test-vendor-version-1.1.2"
	driver := "mydriver"

	endpoint := "test-endpoint"
	nodeID := "test-nodeID"

	//This has to be used as fake cosSession and fake Mounter
	mockSession := &s3client.COSSessionFactory{}
	mockMountObj := &mounter.S3fsMounterFactory{}

	// Creating test logger
	logger, teardown := GetTestLogger(t)
	defer teardown()

	// Setup the CSI driver
	icDriver, err := Setups3Driver(defaultMode, driver, vendorVersion, logger)
	if err != nil {
		t.Fatalf("Failed to setup CSI Driver: %v", err)
	}

	statsUtil := &(utils.VolumeStatsUtils{})
	icsDriver, err := icDriver.NewS3CosDriver(nodeID, endpoint, mockSession, mockMountObj, statsUtil)
	if err != nil {
		t.Fatalf("Failed to create New COS CSI Driver: %v", err)
	}

	return icsDriver
}

func TestSetups3Driver(t *testing.T) {
	// success setting up driver
	driver := inits3Driver(t)
	assert.NotNil(t, driver)

	// Creating test logger
	vendorVersion := "test-vendor-version-1.1.2"
	name := ""
	logger, teardown := GetTestLogger(t)
	defer teardown()

	// Failed setting up driver, name  nil
	_, err := Setups3Driver(defaultMode, name, vendorVersion, logger)
	assert.NotNil(t, err)
}
