package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MountRequest ...
type MountRequest struct {
	Path    string          `json:"path"`
	Bucket  string          `json:"bucket"`
	Mounter string          `json:"mounter"`
	Args    json.RawMessage `json:"args"`
}

// MounterArgs ...
type MounterArgs interface {
	Validate(path string) error
	PopulateArgsSlice(bucket, path string) ([]string, error)
}

func strictDecodeForUnknownFields(data json.RawMessage, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func pathValidator(targetPath string) error {
	if !(strings.HasPrefix(targetPath, "/var/data/kubelet/pods") || strings.HasPrefix(targetPath, "/var/lib/kubelet/pods")) {
		return fmt.Errorf("bad value for target path \"%v\"", targetPath)
	}
	return nil
}

// --- Parser for Mounter Arguments ---

func (req *MountRequest) ParseMounterArgs() ([]string, error) {
	switch req.Mounter {
	case s3fs:
		var args S3FSArgs
		if err := strictDecodeForUnknownFields(req.Args, &args); err != nil {
			return nil, fmt.Errorf("invalid s3fs args decode error: %w", err)
		}
		if err := args.Validate(req.Path); err != nil {
			return nil, fmt.Errorf("s3fs args validation failed: %w", err)
		}
		return args.PopulateArgsSlice(req.Bucket, req.Path)

	case rclone:
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
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}
