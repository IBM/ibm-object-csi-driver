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
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"testing"
)

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

func inits3Driver(t *testing.T) *s3Driver {
	vendorVersion := "test-vendor-version-1.1.2"
	driver := "mydriver"

	endpoint := "test-endpoint"
	nodeID := "test-nodeID"

	// Creating test logger
	logger, teardown := GetTestLogger(t)
	defer teardown()

	// Setup the CSI driver
	icDriver, err := Setups3Driver(logger, driver, vendorVersion)
	if err != nil {
		t.Fatalf("Failed to setup CSI Driver: %v", err)
	}

	icsDriver, err := icDriver.NewS3CosDriver(nodeID, endpoint)
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
	_, err := Setups3Driver(logger, name, vendorVersion)
	assert.NotNil(t, err)
}
