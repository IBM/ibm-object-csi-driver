package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/gin-gonic/gin"
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

// decodeNoDuplicateKeys reads the request body into v, rejecting any JSON
// object that contains the same key more than once at the top level.
// It also applies DisallowUnknownFields so extra envelope fields are caught.
// On error it writes the appropriate 400 response and returns false.
func decodeNoDuplicateKeys(c *gin.Context, v interface{}) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return false
	}

	// First pass: detect duplicate top-level keys using the token stream.
	dec := json.NewDecoder(bytes.NewReader(body))
	// Consume opening '{'.
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: expected JSON object"})
		return false
	}
	seen := make(map[string]struct{})
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: malformed JSON"})
			return false
		}
		key, ok := keyTok.(string)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: non-string key"})
			return false
		}
		if _, exists := seen[key]; exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: duplicate key %q", key)})
			return false
		}
		seen[key] = struct{}{}
		// Skip the value token(s) so the loop advances to the next key.
		if err := skipJSONValue(dec); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: malformed JSON"})
			return false
		}
	}

	// Second pass: unmarshal with DisallowUnknownFields.
	dec2 := json.NewDecoder(bytes.NewReader(body))
	dec2.DisallowUnknownFields()
	if err := dec2.Decode(v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return false
	}
	return true
}

// skipJSONValue advances the decoder past one complete JSON value
// (scalar, object, or array) so the outer loop can move to the next key.
func skipJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch tok {
	case json.Delim('{'), json.Delim('['):
		closing := json.Delim('}')
		if tok == json.Delim('[') {
			closing = json.Delim(']')
		}
		for dec.More() {
			if err := skipJSONValue(dec); err != nil {
				return err
			}
			// For objects, also skip the value after each key.
			if tok == json.Delim('{') {
				if err := skipJSONValue(dec); err != nil {
					return err
				}
			}
		}
		if _, err := dec.Token(); err != nil { // consume closing delimiter
			return err
		}
		_ = closing
	}
	return nil
}

func pathValidator(targetPath string) error {
	absPath, err := absPathResolver(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute mount path: %v", err)
	}
	if !strings.HasPrefix(absPath, safeMountDirs[0]) && !strings.HasPrefix(absPath, safeMountDirs[1]) {
		return fmt.Errorf("bad value for target path \"%v\"", targetPath)
	}
	return nil
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
