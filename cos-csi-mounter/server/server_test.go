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
	err := os.WriteFile(socketPath, []byte("fake"), 0644)
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
	case <-time.After(500 * time.Millisecond):
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
