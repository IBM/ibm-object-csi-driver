package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MountRequest ...
type MountRequest struct {
	Path    string          `json:"path"`
	Bucket  string          `json:"bucket"`
	Mounter string          `json:"mounter"`
	Args    json.RawMessage `json:"args"`
}

var (
	// Directories where bucket can be mounted
	safeMountDirs = []string{"/var/data/kubelet/pods", "/var/lib/kubelet/pods"}
	// Directories where s3fs/rclone configuration files need to be present
	safeMounterConfigDir = "/var/lib/coscsi-config"

	FileExists      = fileExists
	absPathResolver = filepath.Abs
)

// MounterArgs ...
type MounterArgs interface {
	Validate(path string) error
	PopulateArgsSlice(bucket, path string) ([]string, error)
}

type MounterArgsParser interface {
	Parse(request MountRequest) ([]string, error)
}

func strictDecodeForUnknownFields(data json.RawMessage, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// jsonFieldNames returns the exact JSON key names declared on the struct
// that v points to.
func jsonFieldNames(v interface{}) map[string]struct{} {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	names := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			names[name] = struct{}{}
		}
	}
	return names
}

// decodeNoDuplicateKeys reads the request body into v, rejecting any JSON
// object that contains the same key more than once at the top level.
// encoding/json matches keys to struct fields case-insensitively (including
// Unicode folds such as "arg\u017f" == "args"), which would let a
// case-variant key silently collide with a real one. To avoid that, every
// top-level key must exactly match a json tag on v; duplicates are then
// detected with a plain exact-string comparison. It also applies
// DisallowUnknownFields and rejects any data trailing the JSON object.
// On error it writes the appropriate 400 response and returns false.
func decodeNoDuplicateKeys(c *gin.Context, v interface{}) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Error("failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return false
	}

	// First pass: detect duplicate top-level keys using the token stream.
	dec := json.NewDecoder(bytes.NewReader(body))
	// Consume opening '{'.
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		logger.Error("invalid request body: expected JSON object", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: expected JSON object"})
		return false
	}
	allowed := jsonFieldNames(v)
	seen := make(map[string]struct{})
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			logger.Error("invalid request body: failed to read key", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: malformed JSON"})
			return false
		}
		key, ok := keyTok.(string)
		if !ok {
			logger.Error("invalid request body: non-string key")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: non-string key"})
			return false
		}
		if _, ok := allowed[key]; !ok {
			logger.Error("invalid request body: unknown or non-exact key", zap.String("key", key))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return false
		}
		if _, exists := seen[key]; exists {
			logger.Error("invalid request body: duplicate key", zap.String("key", key))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: duplicate key %q", key)})
			return false
		}
		seen[key] = struct{}{}
		// Skip the value so the loop advances to the next key.
		if err := skipJSONValue(dec); err != nil {
			logger.Error("invalid request body: failed to read value", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: malformed JSON"})
			return false
		}
	}

	// Second pass: unmarshal with DisallowUnknownFields.
	dec2 := json.NewDecoder(bytes.NewReader(body))
	dec2.DisallowUnknownFields()
	if err := dec2.Decode(v); err != nil {
		logger.Error("invalid request body: decode failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return false
	}
	// Reject trailing data after the JSON object.
	if _, err := dec2.Token(); err != io.EOF {
		logger.Error("invalid request body: unexpected trailing data")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return false
	}
	return true
}

// skipJSONValue advances the decoder past one complete JSON value
// (scalar, object, or array) so the caller can move to the next key.
func skipJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || (d != '{' && d != '[') {
		return nil // scalar value
	}
	// Composite value: consume tokens until the matching closing delimiter.
	for depth := 1; depth > 0; {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

// pathValidator validates that the target path resolves to an absolute path,
// ends with "/mount", and resides strictly within one of the safe mount directories.
func pathValidator(targetPath string) error {
	absPath, err := absPathResolver(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute mount path: %v", err)
	}
	// CSI target paths must end with "/mount"
	if !strings.HasSuffix(absPath, "/mount") {
		return fmt.Errorf("bad value for target path \"%v\"", targetPath)
	}
	for _, dir := range safeMountDirs {
		// Use dir+"/" so that a path like /var/lib/kubelet/pods-evil does not
		// pass by sharing a string prefix with /var/lib/kubelet/pods.
		if strings.HasPrefix(absPath, dir+"/") {
			return nil
		}
	}
	return fmt.Errorf("bad value for target path \"%v\"", targetPath)
}

// --- Parser for Mounter Arguments ---

type DefaultMounterArgsParser struct{}

func (p *DefaultMounterArgsParser) Parse(request MountRequest) ([]string, error) {
	return request.ParseMounterArgs()
}

func (req *MountRequest) ParseMounterArgs() ([]string, error) {
	switch req.Mounter {
	case constants.S3FS:
		var args S3FSArgs
		if err := strictDecodeForUnknownFields(req.Args, &args); err != nil {
			return nil, fmt.Errorf("invalid s3fs args decode error: %w", err)
		}
		if err := args.Validate(req.Path); err != nil {
			return nil, fmt.Errorf("s3fs args validation failed: %w", err)
		}
		return args.PopulateArgsSlice(req.Bucket, req.Path)

	case constants.RClone:
		var args RCloneArgs
		if err := strictDecodeForUnknownFields(req.Args, &args); err != nil {
			return nil, fmt.Errorf("invalid rclone args decode error: %w", err)
		}
		if err := args.Validate(req.Path); err != nil {
			return nil, fmt.Errorf("rclone args validation failed: %w", err)
		}
		return args.PopulateArgsSlice(req.Bucket, req.Path)

	default:
		return nil, fmt.Errorf("unknown mounter: %s", req.Mounter)
	}
}

// isBoolString checks if a string is "true" or "false" (case-insensitive)
func isBoolString(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "true" || s == "false"
}

// fileExists checks whether the given file path exists and is not a directory.
func fileExists(path string) (bool, error) {
	absPath, err := absPathResolver(path)
	if err != nil {
		return false, fmt.Errorf("failed to resolve absolute path: %v", err)
	}
	if !strings.HasPrefix(absPath, safeMounterConfigDir) {
		return false, fmt.Errorf("path %v is outside the safe directory", absPath)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// redactArgsForLog returns a copy of the raw Args JSON with the value of
// "passwd_file" replaced by "xxxxx", leaving all other fields unchanged.
// If the JSON cannot be parsed, the original raw bytes are returned as-is.
func redactArgsForLog(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	if _, ok := m["passwd_file"]; ok {
		m["passwd_file"] = "xxxxx"
	}
	redacted, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return redacted
}
