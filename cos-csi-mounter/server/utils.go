package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	PopulateArgsSlice(string, string) ([]string, error)
}
type S3FSArgs struct {
	URL                 string `json:"url,omitempty"`
	PasswdFilePath      string `json:"passwd_file,omitempty"`
	UsePathRequestStyle string `json:"use_path_request_style,omitempty"`
	SigV2               string `json:"sigv2,omitempty"`
	AllowOther          string `json:"allow_other,omitempty"`
	MPUmask             string `json:"mp_umask,omitempty"`
	EndPoint            string `json:"endpoint,omitempty"`
	IBMIamAuth          string `json:"ibm_iam_auth,omitempty"`
	IBMIamEndpoint      string `json:"ibm_iam_endpoint,omitempty"`
	DefaultACL          string `json:"default_acl,omitempty"`
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
		if strings.ToLower(strings.TrimSpace(v)) == "true" {
			result = append(result, fmt.Sprintf("%s", k)) // -o, key
		} else {
			result = append(result, fmt.Sprintf("%s=%v", k, v)) // -o, key=value
		}
	}

	return result, nil // [bucket, path, -o, key1=value1, -o, key2=value2]
}

type RCloneArgs struct {
	Endpoint  string `json:"endpoint,omitempty"`
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

func argsValidator(endpoint, targetPath string) error {
	if !(strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://")) {
		return fmt.Errorf("Bad value for COS endpoint \"%v\": scheme is missing. "+
			"Must be of the form http://<hostname> or https://<hostname>", endpoint)
	}
	if !(strings.HasPrefix(targetPath, "/var/data/kubelet/pods") || strings.HasPrefix(targetPath, "/var/lib/kubelet/pods")) {
		return fmt.Errorf("Bad value for target path \"%v\"", targetPath)
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
		if err := argsValidator(args.URL, req.Path); err != nil {
			return nil, fmt.Errorf("s3fs endpoint or target path validation failed: %w", err)
		}
		return args.PopulateArgsSlice(req.Bucket, req.Path)

	case rclone:
		var args RCloneArgs
		if err := strictDecodeForUnknownFields(req.Args, &args); err != nil {
			return nil, fmt.Errorf("invalid rclone args decode error: %w", err)
		}
		if err := argsValidator(args.Endpoint, req.Path); err != nil {
			return nil, fmt.Errorf("rclone endpoint or target path validation failed: %w", err)
		}
		return args.PopulateArgsSlice(req.Bucket, req.Path)

	default:
		return nil, fmt.Errorf("unknown mounter: %s", req.Mounter)
	}
}
