package main

import (
	"fmt"

	"go.uber.org/zap"
)

type s3MounterArgs struct {
	ReadOnly   string `json:"read-only,omitempty"`
	AllowOther string `json:"allow-other,omitempty"`
	UID        string `json:"uid,omitempty"`
	GID        string `json:"gid,omitempty"`
	// Removed: UMask        — mount-s3 has no --umask flag; use --dir-mode/--file-mode via MountOptions
	// Removed: Region       — mount-s3 auto-detects region from AWS_CONFIG_FILE
	// Removed: AwsConfigDir — mount-s3 has no --aws-config-dir flag
	LogLevel    string `json:"log-level,omitempty"` // valid: "debug", "debug-crt", "no-log"
	EndpointURL string `json:"endpoint-url,omitempty"`
	// AWS credential file paths — NOT CLI flags.
	// Worker sets these as AWS_SHARED_CREDENTIALS_FILE and AWS_CONFIG_FILE
	// env vars on the mount-s3 subprocess before invoking it.
	AwsCredentialsFile string `json:"aws-credentials-file,omitempty"`
	AwsConfigFile      string `json:"aws-config-file,omitempty"`
}

// PopulateArgsSlice builds the CLI args slice for mount-s3.
//
// Key rules:
//   - AwsCredentialsFile and AwsConfigFile are NOT CLI flags — use EnvVars() for those
//   - Boolean flags (--allow-other, --read-only) are emitted without a value
//     because mount-s3 does not accept --allow-other=true syntax
//   - LogLevel maps to --debug / --debug-crt / --no-log (mount-s3 has no --log-level)
func (args s3MounterArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
	result := []string{bucket, targetPath}

	// --allow-other: boolean flag, no value
	if args.AllowOther == "true" {
		result = append(result, "--allow-other")
	}

	// --read-only: boolean flag, no value
	if args.ReadOnly == "true" {
		result = append(result, "--read-only")
	}

	// --uid
	if args.UID != "" {
		result = append(result, "--uid="+args.UID)
	}

	// --gid
	if args.GID != "" {
		result = append(result, "--gid="+args.GID)
	}

	// --endpoint-url
	if args.EndpointURL != "" {
		result = append(result, "--endpoint-url="+args.EndpointURL)
	}

	// Log level: mount-s3 has no --log-level flag.
	// Map to the supported flags only.
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

	return result, nil
}

// EnvVars returns the environment variables that must be set on the mount-s3
// subprocess so it can locate the AWS credentials and config files.
// The caller must add these to cmd.Env before cmd.Start().
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

	// allow-other must be a boolean string if set
	if args.AllowOther != "" {
		if isBool := isBoolString(args.AllowOther); !isBool {
			logger.Error("cannot convert value of allow-other into boolean", zap.Any("allow-other", args.AllowOther))
			return fmt.Errorf("cannot convert value of allow-other into boolean: %v", args.AllowOther)
		}
	}

	// AwsCredentialsFile and AwsConfigFile replace the old AwsConfigDir check
	if args.AwsCredentialsFile == "" {
		logger.Error("aws-credentials-file is required for mount-s3")
		return fmt.Errorf("aws-credentials-file is required for mount-s3")
	}

	if args.AwsConfigFile == "" {
		logger.Error("aws-config-file is required for mount-s3")
		return fmt.Errorf("aws-config-file is required for mount-s3")
	}

	// read-only must be a boolean string if set
	if args.ReadOnly != "" {
		if isBool := isBoolString(args.ReadOnly); !isBool {
			logger.Error("cannot convert value of read-only into boolean", zap.Any("read-only", args.ReadOnly))
			return fmt.Errorf("cannot convert value of read-only into boolean: %v", args.ReadOnly)
		}
	}

	return nil
}