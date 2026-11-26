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
	"github.com/stretchr/testify/mock"
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
	router.POST("/mount", handleCosMount(mockMounter, &DefaultMounterArgsParser{}))

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
	assert.Contains(t, w.Body.String(), "invalid args for mounter")
	assert.Contains(t, w.Body.String(), "unknown mounter: invalid")
	mockMounter.AssertNotCalled(t, "FuseMount", mock.Anything, mock.Anything, mock.Anything)
	mockMounter.AssertNotCalled(t, "FuseMountWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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

func newS3MountRequest() MountRequest {
	return MountRequest{
		Bucket:  "my-bucket",
		Path:    "/mnt/test",
		Mounter: constants.AMAZONS3MOUNTER,
		Args:    json.RawMessage(`{"aws-credentials-file":"/var/lib/coscsi-config/x/credentials","aws-config-file":"/var/lib/coscsi-config/x/config"}`),
	}
}

func serveMount(t *testing.T, mockMounter *MockMounterUtils, mockParser *MockMounterArgsParser, request MountRequest) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, mockParser))

	body, _ := json.Marshal(request)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mount", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

// mount-s3 must be started through FuseMountWithEnv with the AWS credential
// and config file paths from the request args, and never through FuseMount.
func TestHandleCosMount_MountS3_Success(t *testing.T) {
	mockMounter := new(MockMounterUtils)
	mockParser := new(MockMounterArgsParser)

	request := newS3MountRequest()
	expectedArgs := []string{"my-bucket", "/mnt/test", "--allow-other"}
	expectedEnv := []string{
		"AWS_SHARED_CREDENTIALS_FILE=/var/lib/coscsi-config/x/credentials",
		"AWS_CONFIG_FILE=/var/lib/coscsi-config/x/config",
	}

	mockParser.On("Parse", request).Return(expectedArgs, nil)
	mockMounter.On("FuseMountWithEnv", request.Path, request.Mounter, expectedArgs, expectedEnv).Return(nil)

	w := serveMount(t, mockMounter, mockParser, request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
	mockMounter.AssertExpectations(t)
	mockMounter.AssertNotCalled(t, "FuseMount", mock.Anything, mock.Anything, mock.Anything)
	mockParser.AssertExpectations(t)
}

func TestHandleCosMount_MountS3_FuseMountWithEnvFails(t *testing.T) {
	mockMounter := new(MockMounterUtils)
	mockParser := new(MockMounterArgsParser)

	request := newS3MountRequest()
	expectedArgs := []string{"my-bucket", "/mnt/test"}
	expectedEnv := []string{
		"AWS_SHARED_CREDENTIALS_FILE=/var/lib/coscsi-config/x/credentials",
		"AWS_CONFIG_FILE=/var/lib/coscsi-config/x/config",
	}

	mockParser.On("Parse", request).Return(expectedArgs, nil)
	mockMounter.On("FuseMountWithEnv", request.Path, request.Mounter, expectedArgs, expectedEnv).Return(fmt.Errorf("mount error"))

	w := serveMount(t, mockMounter, mockParser, request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "mount failed: mount error")
	mockMounter.AssertExpectations(t)
	mockMounter.AssertNotCalled(t, "FuseMount", mock.Anything, mock.Anything, mock.Anything)
}

// s3fs and rclone must keep using FuseMount without env vars.
func TestHandleCosMount_NonS3_DoesNotUseFuseMountWithEnv(t *testing.T) {
	for _, mounter := range []string{constants.S3FS, constants.RClone} {
		t.Run(mounter, func(t *testing.T) {
			mockMounter := new(MockMounterUtils)
			mockParser := new(MockMounterArgsParser)

			request := newS3MountRequest()
			request.Mounter = mounter
			expectedArgs := []string{"my-bucket", "/mnt/test"}

			mockParser.On("Parse", request).Return(expectedArgs, nil)
			mockMounter.On("FuseMount", request.Path, request.Mounter, expectedArgs).Return(nil)

			w := serveMount(t, mockMounter, mockParser, request)

			assert.Equal(t, http.StatusOK, w.Code)
			mockMounter.AssertExpectations(t)
			mockMounter.AssertNotCalled(t, "FuseMountWithEnv", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestBuildS3EnvVars(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		expected []string
	}{
		{
			name: "both files",
			args: `{"aws-credentials-file":"/c/credentials","aws-config-file":"/c/config"}`,
			expected: []string{
				"AWS_SHARED_CREDENTIALS_FILE=/c/credentials",
				"AWS_CONFIG_FILE=/c/config",
			},
		},
		{
			name:     "only credentials file",
			args:     `{"aws-credentials-file":"/c/credentials"}`,
			expected: []string{"AWS_SHARED_CREDENTIALS_FILE=/c/credentials"},
		},
		{
			name:     "only config file",
			args:     `{"aws-config-file":"/c/config"}`,
			expected: []string{"AWS_CONFIG_FILE=/c/config"},
		},
		{
			name:     "empty values are skipped",
			args:     `{"aws-credentials-file":"","aws-config-file":""}`,
			expected: nil,
		},
		{
			name:     "non-string values are skipped",
			args:     `{"aws-credentials-file":true,"aws-config-file":5}`,
			expected: nil,
		},
		{
			name: "non-string fields elsewhere do not drop the credential vars",
			args: `{"log-metrics":true,"aws-credentials-file":"/c/credentials","aws-config-file":"/c/config"}`,
			expected: []string{
				"AWS_SHARED_CREDENTIALS_FILE=/c/credentials",
				"AWS_CONFIG_FILE=/c/config",
			},
		},
		{
			name:     "no file fields",
			args:     `{"allow-other":"true"}`,
			expected: nil,
		},
		{
			name:     "invalid JSON",
			args:     `{"aws-credentials-file"`,
			expected: nil,
		},
		{
			name:     "empty args",
			args:     ``,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, buildS3EnvVars(json.RawMessage(tt.args)))
		})
	}
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
