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
	os.Setenv("TEST_KEY", "test-value")
	defer os.Unsetenv("TEST_KEY")

	val := getEnv("test_key")
	assert.Equal(t, "test-value", val)
}

func TestGetConfigBool(t *testing.T) {
	logger := getZapLogger()
	os.Setenv("DEBUG_TRACE", "true")
	val := getConfigBool("DEBUG_TRACE", false, *logger)
	assert.True(t, val)
	os.Unsetenv("DEBUG_TRACE")

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
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
