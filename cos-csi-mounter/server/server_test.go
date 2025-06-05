package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementation of MounterUtils
type mockMounter struct {
	mock.Mock
}

func (m *mockMounter) FuseMount(path string, mounter string, args []string) error {
	argsCalled := m.Called(path, mounter, args)
	return argsCalled.Error(0)
}

func (m *mockMounter) FuseUnmount(path string) error {
	argsCalled := m.Called(path)
	return argsCalled.Error(0)
}

type dummyListener struct{}

func (d *dummyListener) Accept() (net.Conn, error) { return nil, nil }
func (d *dummyListener) Close() error              { return nil }
func (d *dummyListener) Addr() net.Addr            { return &net.UnixAddr{Name: "dummy", Net: "unix"} }

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

	// Create a dummy socket file
	err := os.WriteFile(socketPath, []byte("dummy"), 0644)
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
		return &dummyListener{}, nil
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

func TestHandleCosMount_InvalidJSON(t *testing.T) {
	mockMounter := new(mockMounter)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mount", bytes.NewBufferString(`invalid-json`))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid request")
}

func TestHandleCosMount_InvalidMounter(t *testing.T) {
	mockMounter := new(mockMounter)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter))

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
	mockMounter := new(mockMounter)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter))

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

func TestHandleCosUnmount_InvalidJSON(t *testing.T) {
	mock := new(mockMounter)
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
	mock := new(mockMounter)
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
	mock := new(mockMounter)
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
