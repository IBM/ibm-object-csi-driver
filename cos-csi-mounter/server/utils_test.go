package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRedactArgsForLog_MasksPasswdFile(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://s3.example.com","passwd_file":"/var/lib/coscsi-config/secret.txt","allow_other":"true"}`)
	result := redactArgsForLog(raw)

	var m map[string]interface{}
	err := json.Unmarshal(result, &m)
	assert.NoError(t, err)
	assert.Equal(t, "xxxxx", m["passwd_file"])
	assert.Equal(t, "https://s3.example.com", m["url"])
	assert.Equal(t, "true", m["allow_other"])
}

func TestRedactArgsForLog_NoPasswdFile(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://s3.example.com","allow_other":"true"}`)
	result := redactArgsForLog(raw)

	var m map[string]interface{}
	err := json.Unmarshal(result, &m)
	assert.NoError(t, err)
	assert.NotContains(t, m, "passwd_file")
	assert.Equal(t, "https://s3.example.com", m["url"])
}

func TestRedactArgsForLog_EmptyInput(t *testing.T) {
	result := redactArgsForLog(json.RawMessage{})
	assert.Empty(t, result)
}

func TestRedactArgsForLog_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	result := redactArgsForLog(raw)
	assert.Equal(t, []byte(`{invalid}`), []byte(result))
}

func TestParse_UnknownMounter(t *testing.T) {
	req := MountRequest{
		Mounter: "unknown",
	}

	mounter := DefaultMounterArgsParser{}
	args, err := mounter.Parse(req)
	assert.Nil(t, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mounter")
}

func TestParseMounterArgs_S3FS_Valid(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.S3FS,
	}
	argsStruct := S3FSArgs{
		URL: testURL,
	}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.NoError(t, err)
	assert.NotNil(t, args)
}

func TestParseMounterArgs_RClone_Valid(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.RClone,
	}
	argsStruct := RCloneArgs{}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.NoError(t, err)
	assert.NotNil(t, args)
}

func TestParseMounterArgs_S3FS_InvalidJSON(t *testing.T) {
	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.S3FS,
		Args:    json.RawMessage(`{"invalid-json"}`),
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
}

func TestParseMounterArgs_RClone_InvalidJSON(t *testing.T) {
	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.RClone,
		Args:    json.RawMessage(`{"invalid-json"}`),
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
}

func TestParseMounterArgs_S3FS_ValidationFails(t *testing.T) {
	req := MountRequest{
		Path:    "invalid-path",
		Bucket:  testBucket,
		Mounter: constants.S3FS,
	}
	argsStruct := S3FSArgs{}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.Nil(t, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "s3fs args validation failed")
}

func TestParseMounterArgs_RClone_ValidationFails(t *testing.T) {
	req := MountRequest{
		Path:    "invalid-path",
		Bucket:  testBucket,
		Mounter: constants.RClone,
	}
	argsStruct := RCloneArgs{}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.Nil(t, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rclone args validation failed")
}

func TestFileExists_FileDoesNotExist(t *testing.T) {
	path := filepath.Join(safeMounterConfigDir, "nonexistent-file")
	exists, err := fileExists(path)
	assert.False(t, exists)
	assert.NoError(t, err)
}

func TestFileExists_OutsideSafeDirectory(t *testing.T) {
	path := "/tmp/unsafe-file"
	exists, err := fileExists(path)
	assert.False(t, exists)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside the safe directory")
}

func TestFileExists_AbsFails(t *testing.T) {
	originalAbs := absPathResolver
	defer func() { absPathResolver = originalAbs }()

	absPathResolver = func(path string) (string, error) {
		return "", errors.New("Failed to resolve absolute path")
	}

	exists, err := fileExists("invalid-path")
	assert.False(t, exists)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve absolute path")
}

// ── decodeNoDuplicateKeys tests (Remediation 2 — report 4078540) ─────────────

func TestDecodeNoDuplicateKeys_ValidMountRequest(t *testing.T) {
	// Positive: well-formed body with no duplicate keys → decodes correctly.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"path":"/var/lib/kubelet/pods/p1/volumes/x","bucket":"my-bucket","mounter":"s3fs","args":{}}`
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))

	var req MountRequest
	ok := decodeNoDuplicateKeys(c, &req)

	assert.True(t, ok)
	assert.Equal(t, "/var/lib/kubelet/pods/p1/volumes/x", req.Path)
	assert.Equal(t, "my-bucket", req.Bucket)
	assert.Equal(t, "s3fs", req.Mounter)
}

func TestDecodeNoDuplicateKeys_DuplicatePathKey_Rejected(t *testing.T) {
	// Negative: the exact attack from the report — duplicate "path" key.
	// Must return HTTP 400 and not decode the victim path into the struct.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"path":"/var/lib/kubelet/pods/legit/volumes/x",` +
		`"path":"/var/lib/kubelet/pods/victim/volumes/pvc-victim/mount",` +
		`"bucket":"b","mounter":"s3fs","args":{}}`
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))

	var req MountRequest
	ok := decodeNoDuplicateKeys(c, &req)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `duplicate key`)
}

func TestDecodeNoDuplicateKeys_DuplicateBucketKey_Rejected(t *testing.T) {
	// Negative: duplicate key other than "path" is also rejected.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"path":"/var/lib/kubelet/pods/p/volumes/x","bucket":"b1","bucket":"b2","mounter":"s3fs","args":{}}`
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))

	var req MountRequest
	ok := decodeNoDuplicateKeys(c, &req)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `duplicate key`)
}

func TestDecodeNoDuplicateKeys_UnknownField_Rejected(t *testing.T) {
	// Negative: unknown top-level field is rejected (DisallowUnknownFields on envelope).
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"path":"/var/lib/kubelet/pods/p/volumes/x","bucket":"b","mounter":"s3fs","args":{},"evil":"injection"}`
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))

	var req MountRequest
	ok := decodeNoDuplicateKeys(c, &req)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid request")
}

func TestDecodeNoDuplicateKeys_InvalidJSON_Rejected(t *testing.T) {
	// Negative: completely broken JSON.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`not-json`))

	var req MountRequest
	ok := decodeNoDuplicateKeys(c, &req)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecodeNoDuplicateKeys_ValidUnmountRequest(t *testing.T) {
	// Positive: unmount body with single "path" key → decodes correctly.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"path":"/var/lib/kubelet/pods/p1/volumes/x"}`
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))

	var req struct {
		Path string `json:"path"`
	}
	ok := decodeNoDuplicateKeys(c, &req)

	assert.True(t, ok)
	assert.Equal(t, "/var/lib/kubelet/pods/p1/volumes/x", req.Path)
}

func TestDecodeNoDuplicateKeys_DuplicatePathInUnmount_Rejected(t *testing.T) {
	// Negative: duplicate "path" in unmount body is also rejected.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"path":"/var/lib/kubelet/pods/p1/volumes/x","path":"/mnt/evil"}`
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))

	var req struct {
		Path string `json:"path"`
	}
	ok := decodeNoDuplicateKeys(c, &req)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `duplicate key`)
}

func TestHandleCosMount_DuplicatePathKey_Rejected(t *testing.T) {
	// End-to-end: the handler itself must return 400 when "path" appears twice.
	gin.SetMode(gin.TestMode)
	mockMounter := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, &MockMounterArgsParser{}))

	body := `{"path":"/var/lib/kubelet/pods/legit/volumes/x",` +
		`"path":"/var/lib/kubelet/pods/victim/volumes/pvc-victim/mount",` +
		`"bucket":"b","mounter":"s3fs","args":{}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mount", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "duplicate key")
	// FuseMount must never have been called.
	mockMounter.AssertNotCalled(t, "FuseMount")
}

func TestHandleCosUnmount_DuplicatePathKey_Rejected(t *testing.T) {
	// End-to-end: unmount handler must also reject duplicate "path".
	gin.SetMode(gin.TestMode)
	mockMounter := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/unmount", handleCosUnmount(mockMounter))

	body := `{"path":"/var/lib/kubelet/pods/p1/volumes/x","path":"/mnt/evil"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/unmount", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "duplicate key")
	mockMounter.AssertNotCalled(t, "FuseUnmount")
}
