package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetOptions_Defaults(t *testing.T) {
	os.Args = []string{"cmd"}
	options := getOptions()

	assert.Equal(t, "unix:/tmp/csi.sock", options.Endpoint)
	assert.Equal(t, "controller", options.ServerMode)
	assert.Equal(t, "host01", options.NodeID)
	assert.Equal(t, "0.0.0.0:9080", options.MetricsAddress)
}

func TestGetEnv(t *testing.T) {
	_ = os.Setenv("TEST_KEY", "test-value")
	defer func() {
		_ = os.Unsetenv("TEST_KEY")
	}()

	val := getEnv("test_key")
	assert.Equal(t, "test-value", val)
}

func TestGetConfigBool(t *testing.T) {
	logger := getZapLogger()
	_ = os.Setenv("DEBUG_TRACE", "true")
	val := getConfigBool("DEBUG_TRACE", false, *logger)
	assert.True(t, val)

	_ = os.Setenv("DEBUG_TRACE", "notbool")
	val = getConfigBool("DEBUG_TRACE", false, *logger)
	assert.False(t, val)
	_ = os.Unsetenv("DEBUG_TRACE")

	val = getConfigBool("DEBUG_TRACE", true, *logger)
	assert.True(t, val)
}

func TestServeMetrics(t *testing.T) {
	logger := getZapLogger()
	addr := "127.0.0.1:19191"

	serveMetrics(addr, logger)

	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get("http://" + addr + "/metrics")
	assert.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
