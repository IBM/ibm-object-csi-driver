package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	PopulateArgsSlice(string, string) ([]string, error)
}
type S3FSArgs struct {
	URL                 string `json:"url,omitempty"`
	PasswdFilePath      string `json:"passwd_file,omitempty"`
	UsePathRequestStyle string `json:"use_path_request_style,omitempty"`
}

func (args S3FSArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
	// Marshal to JSON
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	// Unmarshal into map[string]interface{}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	// Convert to key=value slice
	result := []string{bucket, targetPath}
	for k, v := range m {
		result = append(result, "-o")
		result = append(result, fmt.Sprintf("%s=%v", k, v)) // -o, key=value
	}

	return result, nil // [bucket, path, -o, key1=value1, -o, key2=value2]
}

type RCloneArgs struct {
	BackupDir string `json:"backup-dir,omitempty"`
	Bind      string `json:"bind,omitempty"`
	BWLimit   string `json:"bwlimit,omitempty"`
}

func (args RCloneArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
	// Marshal to JSON
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	// Unmarshal into map[string]interface{}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	// Convert to key=value slice
	result := []string{"mount", bucket, targetPath}
	for k, v := range m {
		result = append(result, fmt.Sprintf("--%s=%v", k, v)) // --key=value
	}

	return result, nil // [mount, bucket, path, --key1=value1, --key2=value2]
}

func strictDecodeForUnknownFields(data json.RawMessage, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// --- Parser for Mounter Arguments ---

func (req *MountRequest) ParseMounterArgs() ([]string, error) {
	switch req.Mounter {
	case s3fs:
		var args S3FSArgs
		if err := strictDecodeForUnknownFields(req.Args, &args); err != nil {
			return nil, fmt.Errorf("invalid s3fs args decode error: %w", err)
		}
		return args.PopulateArgsSlice(req.Bucket, req.Path)

	case rclone:
		var args RCloneArgs
		if err := strictDecodeForUnknownFields(req.Args, &args); err != nil {
			return nil, fmt.Errorf("invalid rclone args decode error: %w", err)
		}
		return args.PopulateArgsSlice(req.Bucket, req.Path)

	default:
		return nil, fmt.Errorf("unknown mounter: %s", req.Mounter)
	}
}
