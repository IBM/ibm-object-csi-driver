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
	// Removed: AwsConfigDir — mount-s3 has no --aws-config-dir flag
	LogLevel    string `json:"log-level,omitempty"` // valid: "debug", "debug-crt", "no-log"
	EndpointURL string `json:"endpoint-url,omitempty"`
	// Region is passed explicitly as --region CLI flag in addition to being written
	// in the AWS config file, to ensure it is always set even if AWS_CONFIG_FILE
	// env var is not propagated correctly to the mount-s3 subprocess.
	Region string `json:"region,omitempty"`
	// AWS credential file paths — NOT CLI flags.
	// Worker sets these as AWS_SHARED_CREDENTIALS_FILE and AWS_CONFIG_FILE
	// env vars on the mount-s3 subprocess before invoking it.
	AwsCredentialsFile string `json:"aws-credentials-file,omitempty"`
	AwsConfigFile      string `json:"aws-config-file,omitempty"`

	MaxThreads          string `json:"max-threads,omitempty"`
	ReadPartSize        string `json:"read-part-size,omitempty"`
	WritePartSize       string `json:"write-part-size,omitempty"`
	MaxThroughputGbps   string `json:"maximum-throughput-gbps,omitempty"`
	UploadChecksums     string `json:"upload-checksums,omitempty"`
	CacheDir            string `json:"cache,omitempty"`
	MaxCacheSize        string `json:"max-cache-size,omitempty"`
	MetadataTTL         string `json:"metadata-ttl,omitempty"`
	NegativeMetadataTTL string `json:"negative-metadata-ttl,omitempty"`
	LogMetrics          bool   `json:"log-metrics,omitempty"`
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

	// --region
	if args.Region != "" {
		result = append(result, "--region="+args.Region)
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

	// --- Performance tuning options ---
	// Only appended when explicitly set. mount-s3 built-in defaults apply otherwise.

	// --max-threads (default: 16)
	if args.MaxThreads != "" {
		result = append(result, "--max-threads="+args.MaxThreads)
	}

	// --read-part-size (default: 8 MiB)
	if args.ReadPartSize != "" {
		result = append(result, "--read-part-size="+args.ReadPartSize)
	}

	// --write-part-size (default: 8 MiB)
	if args.WritePartSize != "" {
		result = append(result, "--write-part-size="+args.WritePartSize)
	}

	// --maximum-throughput-gbps (default: auto-detect on EC2, 10 Gbps elsewhere)
	if args.MaxThroughputGbps != "" {
		result = append(result, "--maximum-throughput-gbps="+args.MaxThroughputGbps)
	}

	// --upload-checksums (default: crc32c)
	if args.UploadChecksums != "" {
		result = append(result, "--upload-checksums="+args.UploadChecksums)
	}

	// --cache + --max-cache-size (default: disabled)
	if args.CacheDir != "" {
		result = append(result, "--cache="+args.CacheDir)
		if args.MaxCacheSize != "" {
			result = append(result, "--max-cache-size="+args.MaxCacheSize)
		}
	}

	// --metadata-ttl (default: minimal, or 60s if --cache is set)
	if args.MetadataTTL != "" {
		result = append(result, "--metadata-ttl="+args.MetadataTTL)
	}

	// --negative-metadata-ttl (default: same as metadata-ttl)
	if args.NegativeMetadataTTL != "" {
		result = append(result, "--negative-metadata-ttl="+args.NegativeMetadataTTL)
	}

	// --log-metrics (default: false) — boolean flag, no value
	if args.LogMetrics {
		result = append(result, "--log-metrics")
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

	// upload-checksums must be a valid value if set.
	// Validated early here so users get a clear error rather than a cryptic mount-s3 failure.
	if args.UploadChecksums != "" && args.UploadChecksums != "crc32c" && args.UploadChecksums != "off" {
		logger.Error("invalid upload-checksums value", zap.String("value", args.UploadChecksums))
		return fmt.Errorf("invalid upload-checksums value '%s': must be 'crc32c' or 'off'", args.UploadChecksums)
	}

	return nil
}
