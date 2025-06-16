package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
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

func pathValidator(targetPath string) error {
	absPath, err := filepath.Abs(targetPath)
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

var FileExists = fileExists

// fileExists checks whether the given file path exists and is not a directory.
func fileExists(path string) (bool, error) {
	absPath, err := filepath.Abs(path)
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
