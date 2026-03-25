package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSetupSocket_CreatesSocket(t *testing.T) {
	// Use temp dir for socket dir
	tmpDir := t.TempDir()
	constants.SocketDir = tmpDir
	constants.SocketFile = "test.sock"
	socketPath := filepath.Join(constants.SocketDir, constants.SocketFile)

	listener, err := setupSocket()
	defer func() {
		if listener != nil {
			_ = listener.Close()
		}
		_ = os.Remove(socketPath)
	}()

	assert.NoError(t, err, "expected no error from setupSocket")
	assert.FileExists(t, socketPath, "expected socket file to be created")
}

func TestSetupSocket_MkdirAllFails(t *testing.T) {
	original := MakeDir
	defer func() { MakeDir = original }()

	MakeDir = func(path string, perm os.FileMode) error {
		return errors.New("mock mkdir failure")
	}

	listener, err := setupSocket()
	assert.Nil(t, listener)
	assert.Error(t, err)
}

func TestSetupSocket_FailsToCreateSocket(t *testing.T) {
	tmpDir := t.TempDir()
	constants.SocketDir = tmpDir
	constants.SocketFile = "existing.sock"
	socketPath := filepath.Join(tmpDir, constants.SocketFile)

	// Create a fake socket file
	err := os.WriteFile(socketPath, []byte("fake"), 0600)
	assert.NoError(t, err)

	// Mock removeFile to simulate failure
	originalRemove := RemoveFile
	RemoveFile = func(name string) error {
		return errors.New("mock remove failure")
	}
	defer func() { RemoveFile = originalRemove }()

	// Also inject working listenUnix to avoid interfering errors
	originalListen := UnixSocketListener
	UnixSocketListener = func(network, address string) (net.Listener, error) {
		return &fakeListener{}, nil
	}
	defer func() { UnixSocketListener = originalListen }()

	listener, err := setupSocket()

	assert.NotNil(t, listener)
	assert.NoError(t, err)
}

func TestSetupSocket_StatSocketFileFails(t *testing.T) {
	tmpDir := t.TempDir()
	originalSocketDir := constants.SocketDir
	originalSocketFile := constants.SocketFile
	constants.SocketDir = tmpDir
	constants.SocketFile = string([]byte{0x00}) // Invalid filename on Unix
	defer func() {
		constants.SocketDir = originalSocketDir
		constants.SocketFile = originalSocketFile
	}()

	// Call setupSocket and expect an error
	listener, err := setupSocket()
	assert.Nil(t, listener)
	assert.Error(t, err)
}

func TestNewRouter_HasExpectedRoutes(t *testing.T) {
	router := newRouter()
	assert.NotNil(t, router)
}

func TestStartService(t *testing.T) {
	// Create a channel to receive the error from the goroutine
	errCh := make(chan error, 1)

	go func() {
		err := startService(fakeSetupSocketSuccess, fakeRouter(), fakeHandleSignals)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "simulated shutdown")
	case <-time.After(constants.Interval):
		t.Fatal("startService did not return in time")
	}
}

func TestStartService_SocketError(t *testing.T) {
	err := startService(fakeSetupSocketFail, fakeRouter(), fakeHandleSignals)
	if err == nil {
		t.Error("Expected socket creation error, got nil")
	}
}

func TestHandleCosMount_InvalidJSON(t *testing.T) {
	mockMounter := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, &MockMounterArgsParser{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mount", bytes.NewBufferString(`invalid-json`))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid request")
}

func TestHandleCosMount_InvalidMounter(t *testing.T) {
	mockMounter := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, &MockMounterArgsParser{}))

	reqBody := map[string]interface{}{
		"mounter": "invalid",
		"bucket":  "mybucket",
		"path":    "/mnt/data",
		"args":    json.RawMessage(`{"endpoint": "s3.test"}`),
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mount", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid mounter")
}

func TestHandleCosMount_MissingBucket(t *testing.T) {
	mockMounter := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, &MockMounterArgsParser{}))

	reqBody := map[string]interface{}{
		"mounter": constants.S3FS,
		"path":    "/mnt/data",
		"args":    json.RawMessage(`{"endpoint": "s3.test"}`),
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mount", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "missing bucket")
}

func TestHandleCosMount_InvalidMounterArgs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockMounter := new(MockMounterUtils)
	mockParser := new(MockMounterArgsParser)

	reqBody := []byte(`{
		"bucket": "my-bucket",
		"path": "/mnt/test",
		"mounter": "s3fs",
		"args": {"flag": "--invalid"}
	}`)

	var request MountRequest
	err := json.Unmarshal(reqBody, &request)
	assert.NoError(t, err)

	mockParser.On("Parse", request).Return([]string(nil), fmt.Errorf("invalid arg format"))

	req, _ := http.NewRequest("POST", "/mount", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, mockParser))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid args for mounter")
	mockParser.AssertExpectations(t)
}

func TestHandleCosMount_FuseMountFails(t *testing.T) {
	mockMounter := new(MockMounterUtils)
	mockParser := new(MockMounterArgsParser)

	request := MountRequest{
		Bucket:  "my-bucket",
		Path:    "/mnt/test",
		Mounter: constants.S3FS,
		Args:    json.RawMessage(`["--endpoint=https://s3.example.com"]`),
	}

	expectedArgs := []string{"--endpoint=https://s3.example.com"}

	mockParser.On("Parse", request).Return(expectedArgs, nil)
	mockMounter.On("FuseMount", request.Path, request.Mounter, expectedArgs).Return(fmt.Errorf("mount error"))

	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, mockParser))

	body, _ := json.Marshal(request)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mount", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "mount failed: mount error")

	mockMounter.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestHandleCosMount_Success(t *testing.T) {
	mockMounter := new(MockMounterUtils)
	mockParser := new(MockMounterArgsParser)

	request := MountRequest{
		Bucket:  "my-bucket",
		Path:    "/mnt/test",
		Mounter: constants.S3FS,
		Args:    json.RawMessage(`["--endpoint=https://s3.example.com"]`),
	}

	expectedArgs := []string{"--endpoint=https://s3.example.com"}

	mockParser.On("Parse", request).Return(expectedArgs, nil)
	mockMounter.On("FuseMount", request.Path, request.Mounter, expectedArgs).Return(nil)

	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, mockParser))

	body, _ := json.Marshal(request)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mount", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")

	mockMounter.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestHandleCosUnmount_InvalidJSON(t *testing.T) {
	mock := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/unmount", handleCosUnmount(mock))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/unmount", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid request")
}

func TestHandleCosUnmount_UnmountFailure(t *testing.T) {
	mock := new(MockMounterUtils)
	mock.On("FuseUnmount", "/mnt/fail").Return(errors.New("mock failure"))

	router := gin.Default()
	router.POST("/unmount", handleCosUnmount(mock))

	reqBody := map[string]string{"path": "/mnt/fail"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/unmount", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "unmount failed")
	mock.AssertExpectations(t)
}

func TestHandleCosUnmount_Success(t *testing.T) {
	mock := new(MockMounterUtils)
	mock.On("FuseUnmount", "/mnt/success").Return(nil)

	router := gin.Default()
	router.POST("/unmount", handleCosUnmount(mock))

	reqBody := map[string]string{"path": "/mnt/success"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/unmount", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
	mock.AssertExpectations(t)
}

func TestPerformHealthCheck_Success(t *testing.T) {
	tmpDir := "/tmp"
	constants.SocketDir = tmpDir
	constants.SocketFile = "test-health-success.sock"
	socketPath := filepath.Join(tmpDir, constants.SocketFile)

	os.Remove(socketPath)
	defer os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("Cannot create Unix socket on this platform: %v", err)
		return
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	time.Sleep(50 * time.Millisecond)
	err = performHealthCheck()
	assert.NoError(t, err, "health check should succeed when socket is accessible")
}

func TestPerformHealthCheck_SocketNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	constants.SocketDir = tmpDir
	constants.SocketFile = "nonexistent.sock"

	err := performHealthCheck()
	assert.Error(t, err, "health check should fail when socket doesn't exist")
	assert.Contains(t, err.Error(), "socket file not accessible")
}

func TestPerformHealthCheck_SocketNotConnectable(t *testing.T) {
	tmpDir := "/tmp"
	constants.SocketDir = tmpDir
	constants.SocketFile = "test-unconnectable.sock"
	socketPath := filepath.Join(tmpDir, constants.SocketFile)

	os.Remove(socketPath)
	defer os.Remove(socketPath)

	file, err := os.Create(socketPath)
	assert.NoError(t, err)
	file.Close()

	err = performHealthCheck()
	assert.Error(t, err, "health check should fail when socket is not connectable")
	assert.Contains(t, err.Error(), "failed to connect to socket")
}


func TestWatchdogLoop_NotEnabled(t *testing.T) {
	originalWatchdog := os.Getenv("WATCHDOG_USEC")
	os.Unsetenv("WATCHDOG_USEC")
	defer func() {
		if originalWatchdog != "" {
			os.Setenv("WATCHDOG_USEC", originalWatchdog)
		}
	}()

	done := make(chan bool, 1)
	go func() {
		watchdogLoop()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("watchdogLoop should return immediately when not enabled")
	}
}

func TestWatchdogLoop_Enabled(t *testing.T) {
	os.Setenv("WATCHDOG_USEC", "30000000")
	defer os.Unsetenv("WATCHDOG_USEC")

	tmpDir := "/tmp"
	constants.SocketDir = tmpDir
	constants.SocketFile = "test-watchdog.sock"
	socketPath := filepath.Join(tmpDir, constants.SocketFile)

	os.Remove(socketPath)
	defer os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("Cannot create Unix socket on this platform: %v", err)
		return
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	done := make(chan bool, 1)
	go func() {
		watchdogLoop()
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("watchdogLoop should not return immediately when enabled")
	case <-time.After(200 * time.Millisecond):
	}

	listener.Close()
}

func TestWatchdogLoop_HealthCheckFails(t *testing.T) {
	os.Setenv("WATCHDOG_USEC", "1000000")
	defer os.Unsetenv("WATCHDOG_USEC")

	tmpDir := t.TempDir()
	constants.SocketDir = tmpDir
	constants.SocketFile = "fail-health.sock"

	done := make(chan bool, 1)
	go func() {
		watchdogLoop()
		done <- true
	}()

	time.Sleep(600 * time.Millisecond)

	select {
	case <-done:
		t.Fatal("watchdogLoop should continue running even when health checks fail")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPerformHealthCheck_ConcurrentCalls(t *testing.T) {
	tmpDir := "/tmp"
	constants.SocketDir = tmpDir
	constants.SocketFile = "test-concurrent.sock"
	socketPath := filepath.Join(tmpDir, constants.SocketFile)

	os.Remove(socketPath)
	defer os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("Cannot create Unix socket on this platform: %v", err)
		return
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	time.Sleep(50 * time.Millisecond)
	const numConcurrent = 10
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func() {
			errors <- performHealthCheck()
		}()
	}
	for i := 0; i < numConcurrent; i++ {
		err := <-errors
		assert.NoError(t, err, "concurrent health check %d should succeed", i)
	}
}
