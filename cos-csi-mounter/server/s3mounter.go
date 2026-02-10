package main

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

type s3MounterArgs struct {
	ReadOnly   string `json:"read-only,omitempty"`
	AllowOther string `json:"allow-other,omitempty"`
	UID        string `json:"uid,omitempty"`
	GID        string `json:"gid,omitempty"`
	UMask      string `json:"umask,omitempty"`
	LogLevel   string `json:"log-level,omitempty"`
}

func (args s3MounterArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
	// Marshal to JSON
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	// Unmarshal into map[string]string
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	// Convert to key=value slice
	result := []string{"mount", bucket, targetPath}
	for k, v := range m {
		result = append(result, fmt.Sprintf("--%s=%v", k, v)) // --key=value
	}

	return result, nil // [mount, bucket, path, --key1=value1, --key2=value2, ...]
}

func (args s3MounterArgs) Validate(targetPath string) error {
	if err := pathValidator(targetPath); err != nil {
		return err
	}

	// Check if value of allow-other parameter is boolean "true" or "false"
	if args.AllowOther != "" {
		if isBool := isBoolString(args.AllowOther); !isBool {
			logger.Error("cannot convert value of allow-other into boolean", zap.Any("allow-other", args.AllowOther))
			return fmt.Errorf("cannot convert value of allow-other into boolean: %v", args.AllowOther)
		}
	}

	// // Check if rclone config file exists or not
	// if exists, err := FileExists(args.ConfigPath); err != nil {
	// 	logger.Error("error checking rclone config file existence")
	// 	return fmt.Errorf("error checking rclone config file existence")
	// } else if !exists {
	// 	logger.Error("rclone config file not found")
	// 	return fmt.Errorf("rclone config file not found")
	// }


	// Check if value of read-only parameter is boolean "true" or "false"
	if args.ReadOnly != "" {
		if isBool := isBoolString(args.ReadOnly); !isBool {
			logger.Error("cannot convert value of read-only into boolean", zap.Any("read-only", args.ReadOnly))
			return fmt.Errorf("cannot convert value of read-only into boolean: %v", args.ReadOnly)
		}
	}

	return nil
}
