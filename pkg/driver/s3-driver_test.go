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

	// Setup the IBM CSI driver
	icDriver, err := Setups3Driver(logger, driver, vendorVersion)
	if err != nil {
		t.Fatalf("Failed to setup IBM CSI Driver: %v", err)
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
