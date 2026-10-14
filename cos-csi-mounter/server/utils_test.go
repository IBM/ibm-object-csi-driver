package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

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

// ── exact-key, trailing-data and skipJSONValue tests ─────────────────────────

// newDecodeCtx builds a gin test context whose request body is the given reader.
func newDecodeCtx(body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", body)
	return c, w
}

func TestDecodeNoDuplicateKeys_NonExactKey_Rejected(t *testing.T) {
	// Negative: encoding/json matches keys case-insensitively and with Unicode
	// folds. Any key that is not an exact match for a json tag must be rejected,
	// whether or not the exact key is also present.
	bodies := []string{
		`{"path":"/var/lib/kubelet/pods/legit/volumes/x","PATH":"/var/lib/kubelet/pods/victim/volumes/pvc/mount"}`,
		`{"PATH":"/var/lib/kubelet/pods/p/volumes/x"}`,
		`{"path":"/var/lib/kubelet/pods/p/volumes/x","bucket":"a","buc\u212aet":"b"}`, // Kelvin sign
		`{"path":"/var/lib/kubelet/pods/p/volumes/x","args":{},"arg\u017f":{}}`,       // long s
	}
	for _, body := range bodies {
		c, w := newDecodeCtx(strings.NewReader(body))

		var req MountRequest
		ok := decodeNoDuplicateKeys(c, &req)

		assert.False(t, ok, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Contains(t, w.Body.String(), "invalid request", body)
	}
}

func TestDecodeNoDuplicateKeys_NonExactKeyInUnmount_Rejected(t *testing.T) {
	// Negative: the exact-key rule also applies to the anonymous unmount struct.
	c, w := newDecodeCtx(strings.NewReader(`{"path":"/var/lib/kubelet/pods/p/volumes/x","Path":"/mnt/evil"}`))

	var req struct {
		Path string `json:"path"`
	}
	ok := decodeNoDuplicateKeys(c, &req)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecodeNoDuplicateKeys_TrailingData_Rejected(t *testing.T) {
	// Negative: any data after the JSON object must be rejected.
	bodies := []string{
		`{"path":"/var/lib/kubelet/pods/p/volumes/x"}{"path":"/mnt/evil"}`,
		`{"path":"/var/lib/kubelet/pods/p/volumes/x"} trailing`,
		`{"path":"/var/lib/kubelet/pods/p/volumes/x"} []`,
	}
	for _, body := range bodies {
		c, w := newDecodeCtx(strings.NewReader(body))

		var req struct {
			Path string `json:"path"`
		}
		ok := decodeNoDuplicateKeys(c, &req)

		assert.False(t, ok, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
	}
}

func TestDecodeNoDuplicateKeys_TrailingWhitespace_Accepted(t *testing.T) {
	// Positive: trailing whitespace/newlines are not data and must still pass.
	c, w := newDecodeCtx(strings.NewReader("{\"path\":\"/var/lib/kubelet/pods/p/volumes/x\"}\n  \n"))

	var req struct {
		Path string `json:"path"`
	}
	ok := decodeNoDuplicateKeys(c, &req)

	assert.True(t, ok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/var/lib/kubelet/pods/p/volumes/x", req.Path)
}

func TestDecodeNoDuplicateKeys_NestedValuesSkipped(t *testing.T) {
	// Positive: nested objects/arrays are skipped correctly, and keys repeated
	// inside nested values are not treated as top-level duplicates.
	body := `{"path":"/var/lib/kubelet/pods/p/volumes/x","bucket":"b","mounter":"s3fs",` +
		`"args":{"a":[1,{"path":"n"},[2,3]],"path":"m"}}`
	c, _ := newDecodeCtx(strings.NewReader(body))

	var req MountRequest
	ok := decodeNoDuplicateKeys(c, &req)

	assert.True(t, ok)
	assert.Equal(t, "/var/lib/kubelet/pods/p/volumes/x", req.Path)
	assert.Equal(t, "b", req.Bucket)
	assert.JSONEq(t, `{"a":[1,{"path":"n"},[2,3]],"path":"m"}`, string(req.Args))
}

func TestDecodeNoDuplicateKeys_NotAnObject_Rejected(t *testing.T) {
	// Negative: top-level value must be a JSON object.
	for _, body := range []string{``, `null`, `[]`, `"str"`, `123`} {
		c, w := newDecodeCtx(strings.NewReader(body))

		var req MountRequest
		ok := decodeNoDuplicateKeys(c, &req)

		assert.False(t, ok, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Contains(t, w.Body.String(), "expected JSON object", body)
	}
}

func TestDecodeNoDuplicateKeys_TruncatedValue_Rejected(t *testing.T) {
	// Negative: bodies that end inside a value or before the closing brace.
	bodies := []string{
		`{"path"`,
		`{"path":`,
		`{"path":{"a":`,
		`{"path":"x","args":[1,[2`,
		`{"path":"x"`,
	}
	for _, body := range bodies {
		c, w := newDecodeCtx(strings.NewReader(body))

		var req MountRequest
		ok := decodeNoDuplicateKeys(c, &req)

		assert.False(t, ok, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
	}
}

func TestDecodeNoDuplicateKeys_ReadError_Rejected(t *testing.T) {
	// Negative: failure while reading the request body.
	c, w := newDecodeCtx(iotest.ErrReader(errors.New("boom")))

	var req MountRequest
	ok := decodeNoDuplicateKeys(c, &req)

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "failed to read request body")
	// Internal error detail must not leak to the client.
	assert.NotContains(t, w.Body.String(), "boom")
}

func TestJSONFieldNames(t *testing.T) {
	type sample struct {
		Plain    string `json:"plain"`
		WithOpt  string `json:"with_opt,omitempty"`
		Skipped  string `json:"-"`
		NoName   string `json:",omitempty"`
		Untagged string
	}

	// Pointer to struct.
	assert.Equal(t,
		map[string]struct{}{"plain": {}, "with_opt": {}},
		jsonFieldNames(&sample{}))

	// Pointer to pointer to struct.
	p := &sample{}
	assert.Equal(t,
		map[string]struct{}{"plain": {}, "with_opt": {}},
		jsonFieldNames(&p))

	// The real request types.
	assert.Equal(t,
		map[string]struct{}{"path": {}, "bucket": {}, "mounter": {}, "args": {}},
		jsonFieldNames(&MountRequest{}))
}

func TestSkipJSONValue(t *testing.T) {
	// Input is `{"a":<value>,"b":1}`. After consuming '{' and "a", skipJSONValue
	// must consume exactly <value>, leaving "b" as the next token.
	values := []string{
		`"str"`,
		`123`,
		`true`,
		`null`,
		`{}`,
		`[]`,
		`{"x":1,"x":2}`,
		`[1,2,3]`,
		`{"x":[1,{"y":[2,{"z":{}}]}],"w":"v"}`,
		`[[[[]]]]`,
		`[{"a":[]},{"b":{}}]`,
	}
	for _, v := range values {
		dec := json.NewDecoder(strings.NewReader(`{"a":` + v + `,"b":1}`))
		_, err := dec.Token() // '{'
		assert.NoError(t, err, v)
		_, err = dec.Token() // "a"
		assert.NoError(t, err, v)

		assert.NoError(t, skipJSONValue(dec), v)

		tok, err := dec.Token()
		assert.NoError(t, err, v)
		assert.Equal(t, "b", tok, v)
	}
}

func TestSkipJSONValue_Errors(t *testing.T) {
	// Empty input and values truncated at various nesting depths must error.
	inputs := []string{
		``,
		`{`,
		`[`,
		`[1,2`,
		`{"a":1`,
		`[[1,2]`,
		`{"a":[1,{"b":2}`,
		`]`,
	}
	for _, in := range inputs {
		dec := json.NewDecoder(strings.NewReader(in))
		assert.Error(t, skipJSONValue(dec), in)
	}
}

func TestHandleCosMount_NonExactKey_Rejected(t *testing.T) {
	// End-to-end: handler returns 400 for a case-variant key and never mounts.
	gin.SetMode(gin.TestMode)
	mockMounter := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/mount", handleCosMount(mockMounter, &MockMounterArgsParser{}))

	body := `{"path":"/var/lib/kubelet/pods/legit/volumes/x",` +
		`"PATH":"/var/lib/kubelet/pods/victim/volumes/pvc-victim/mount",` +
		`"bucket":"b","mounter":"s3fs","args":{}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mount", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockMounter.AssertNotCalled(t, "FuseMount")
}

func TestHandleCosUnmount_TrailingData_Rejected(t *testing.T) {
	// End-to-end: handler returns 400 for a second appended JSON object.
	gin.SetMode(gin.TestMode)
	mockMounter := new(MockMounterUtils)
	router := gin.Default()
	router.POST("/unmount", handleCosUnmount(mockMounter))

	body := `{"path":"/var/lib/kubelet/pods/p1/volumes/x"}{"path":"/mnt/evil"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/unmount", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockMounter.AssertNotCalled(t, "FuseUnmount")
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

// ── pathValidator tests (Remediation 3 — report 4078540) ─────────────────────

func TestPathValidator_LegitPodPath_Passes(t *testing.T) {
	// Positive: a normal pod volume path under /var/lib/kubelet/pods.
	err := pathValidator("/var/lib/kubelet/pods/abc-123/volumes/kubernetes.io~csi/pvc/mount")
	assert.NoError(t, err)
}

func TestPathValidator_AltSafeDirPath_Passes(t *testing.T) {
	// Positive: path under the second safe directory /var/data/kubelet/pods.
	err := pathValidator("/var/data/kubelet/pods/abc-123/volumes/kubernetes.io~csi/pvc/mount")
	assert.NoError(t, err)
}

func TestPathValidator_ExactSafeDir_Rejected(t *testing.T) {
	// Negative: the safe directory itself does not end with /mount.
	err := pathValidator("/var/lib/kubelet/pods")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestPathValidator_MissingMountSuffix_Rejected(t *testing.T) {
	// Negative: valid prefix under safe dir but does not end with /mount.
	err := pathValidator("/var/lib/kubelet/pods/abc-123/volumes/kubernetes.io~csi/pvc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestPathValidator_PodsEvilSuffix_Rejected(t *testing.T) {
	// Negative: the exact bypass from the report — /var/lib/kubelet/pods-evil
	// shares the string prefix "/var/lib/kubelet/pods" but is a different directory.
	// Before the fix this passed; after the fix it must be rejected.
	err := pathValidator("/var/lib/kubelet/pods-evil/volumes/x/mount")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestPathValidator_PodsEvilSuffixAltDir_Rejected(t *testing.T) {
	// Same bypass attempt against the /var/data/kubelet/pods safe dir.
	err := pathValidator("/var/data/kubelet/pods-evil/volumes/x/mount")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestPathValidator_EtcPath_Rejected(t *testing.T) {
	// Negative: completely out-of-bounds path.
	err := pathValidator("/etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestPathValidator_RootPath_Rejected(t *testing.T) {
	// Negative: filesystem root.
	err := pathValidator("/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestPathValidator_AbsResolverFails_ReturnsError(t *testing.T) {
	// Negative: absPathResolver returns an error → pathValidator propagates it.
	orig := absPathResolver
	defer func() { absPathResolver = orig }()

	absPathResolver = func(string) (string, error) {
		return "", fmt.Errorf("mock resolve failure")
	}

	err := pathValidator("/var/lib/kubelet/pods/x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve absolute mount path")
}
