package main

import (
	"fmt"

	"go.uber.org/zap"
)

// s3MounterArgs holds the args sent from the node server for mount-s3.
// This struct must match the s3MounterArgs in pkg/mounter/mounter_s3mounter.go
type s3MounterArgs struct {
	// Always set.
	AllowOther         string `json:"allow-other,omitempty"`
	AwsCredentialsFile string `json:"aws-credentials-file,omitempty"`
	AwsConfigFile      string `json:"aws-config-file,omitempty"`

	// SecretMap-sourced.
	EndpointURL string `json:"endpoint-url,omitempty"`
	Region      string `json:"region,omitempty"`

	// Identity.
	UID string `json:"uid,omitempty"`
	GID string `json:"gid,omitempty"`

	// Logging (translated from LogLevel).
	LogLevel     string `json:"log-level,omitempty"`
	LogDirectory string `json:"log-directory,omitempty"`

	// Cache.
	CacheDir string `json:"cache,omitempty"`

	// Write operations - conflict resolution applied when read-only is set.
	ReadOnly          string `json:"read-only,omitempty"`
	AllowOverwrite    string `json:"allow-overwrite,omitempty"`
	IncrementalUpload string `json:"incremental-upload,omitempty"`

	// Passthrough flags — user-supplied flags not handled by structured fields.
	// Secret flags override SC flags when the same key appears in both.
	// These are already deduplicated by the node server before being sent here.
	Args []string `json:"args,omitempty"`
}

// PopulateArgsSlice builds the CLI args slice for mount-s3.
func (args s3MounterArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
	result := []string{bucket, targetPath}

	// Block incremental-upload flag (causes data loss with IBM COS).
	// This flag uses multiple PutObject calls that overwrite each other,
	// resulting in ~75% data loss.
	if args.IncrementalUpload == "true" {
		logger.Warn("incremental-upload is not supported with IBM Cloud Object Storage, disabling")
		args.IncrementalUpload = ""
	}

	// Read-only priority resolution.
	if args.ReadOnly == "true" {
		if args.AllowOverwrite == "true" {
			logger.Warn("read-only is set, clearing allow-overwrite")
			args.AllowOverwrite = ""
		}
		if args.IncrementalUpload == "true" {
			logger.Warn("read-only is set, clearing incremental-upload")
			args.IncrementalUpload = ""
		}
	}

	if args.AllowOther == "true" {
		result = append(result, "--allow-other")
	}
	if args.ReadOnly == "true" {
		result = append(result, "--read-only")
	}
	if args.AllowOverwrite == "true" {
		result = append(result, "--allow-overwrite")
	}
	if args.IncrementalUpload == "true" {
		result = append(result, "--incremental-upload")
	}
	if args.UID != "" {
		result = append(result, "--uid="+args.UID)
	}
	if args.GID != "" {
		result = append(result, "--gid="+args.GID)
	}
	if args.EndpointURL != "" {
		result = append(result, "--endpoint-url="+args.EndpointURL)
	}
	if args.Region != "" {
		result = append(result, "--region="+args.Region)
	}

	switch args.LogLevel {
	case "debug":
		result = append(result, "--debug")
	case "debug-crt":
		result = append(result, "--debug-crt")
	case "no-log":
		result = append(result, "--no-log")
	default:
		if args.LogLevel != "" {
			logger.Warn("unsupported log level for mount-s3, ignoring",
				zap.String("log-level", args.LogLevel),
				zap.String("supported", "debug, debug-crt, no-log"),
			)
		}
	}

	if args.LogDirectory != "" {
		result = append(result, "--log-directory="+args.LogDirectory)
	}
	if args.CacheDir != "" {
		result = append(result, "--cache="+args.CacheDir)
	}

	// Passthrough flags appended last — already deduplicated by node server.
	result = append(result, args.Args...)

	return result, nil
}

// EnvVars returns the environment variables for the mount-s3 subprocess.
func (args s3MounterArgs) EnvVars() []string {
	var envVars []string
	if args.AwsCredentialsFile != "" {
		envVars = append(envVars, "AWS_SHARED_CREDENTIALS_FILE="+args.AwsCredentialsFile)
	}
	if args.AwsConfigFile != "" {
		envVars = append(envVars, "AWS_CONFIG_FILE="+args.AwsConfigFile)
	}
	return envVars
}

// Validate checks that required fields are present and values are valid.
func (args s3MounterArgs) Validate(targetPath string) error {
	if err := pathValidator(targetPath); err != nil {
		return err
	}

	if args.AllowOther != "" {
		if isBool := isBoolString(args.AllowOther); !isBool {
			logger.Error("cannot convert value of allow-other into boolean", zap.Any("allow-other", args.AllowOther))
			return fmt.Errorf("cannot convert value of allow-other into boolean: %v", args.AllowOther)
		}
	}

	if args.AwsCredentialsFile == "" {
		logger.Error("aws-credentials-file is required for mount-s3")
		return fmt.Errorf("aws-credentials-file is required for mount-s3")
	}
	if args.AwsConfigFile == "" {
		logger.Error("aws-config-file is required for mount-s3")
		return fmt.Errorf("aws-config-file is required for mount-s3")
	}

	if args.ReadOnly != "" {
		if isBool := isBoolString(args.ReadOnly); !isBool {
			logger.Error("cannot convert value of read-only into boolean", zap.Any("read-only", args.ReadOnly))
			return fmt.Errorf("cannot convert value of read-only into boolean: %v", args.ReadOnly)
		}
	}
	if args.AllowOverwrite != "" {
		if isBool := isBoolString(args.AllowOverwrite); !isBool {
			logger.Error("cannot convert value of allow-overwrite into boolean", zap.Any("allow-overwrite", args.AllowOverwrite))
			return fmt.Errorf("cannot convert value of allow-overwrite into boolean: %v", args.AllowOverwrite)
		}
	}
	if args.IncrementalUpload != "" {
		if isBool := isBoolString(args.IncrementalUpload); !isBool {
			logger.Error("cannot convert value of incremental-upload into boolean", zap.Any("incremental-upload", args.IncrementalUpload))
			return fmt.Errorf("cannot convert value of incremental-upload into boolean: %v", args.IncrementalUpload)
		}
	}

	// ensureDir creates the directory if it does not exist.
	if args.CacheDir != "" {
		if err := ensureDir(args.CacheDir); err != nil {
			logger.Error("failed to create cache directory", zap.String("cache-dir", args.CacheDir), zap.Error(err))
			return fmt.Errorf("failed to create cache directory '%s': %w", args.CacheDir, err)
		}
	}
	if args.LogDirectory != "" {
		if err := ensureDir(args.LogDirectory); err != nil {
			logger.Error("failed to create log directory", zap.String("log-dir", args.LogDirectory), zap.Error(err))
			return fmt.Errorf("failed to create log directory '%s': %w", args.LogDirectory, err)
		}
	}

	return nil
}