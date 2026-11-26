package main

import (
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"
)

var (
	s3MounterSafeLogDir   = "/var/log/mount-s3"
	s3MounterSafeCacheDir = "/tmp"
)

// s3MounterArgs must match s3MounterArgs in pkg/mounter/mounter-mount-s3.go.
type s3MounterArgs struct {
	AllowOther         string `json:"allow-other,omitempty"`
	AwsCredentialsFile string `json:"aws-credentials-file,omitempty"`
	AwsConfigFile      string `json:"aws-config-file,omitempty"`
	EndpointURL        string `json:"endpoint-url,omitempty"`
	Region             string `json:"region,omitempty"`
	UID                string `json:"uid,omitempty"`
	GID                string `json:"gid,omitempty"`
	LogLevel           string `json:"log-level,omitempty"`
	LogDirectory       string `json:"log-directory,omitempty"`
	CacheDir           string `json:"cache,omitempty"`
	ReadOnly           string `json:"read-only,omitempty"`
	AllowDelete        string `json:"allow-delete,omitempty"`
	AllowOverwrite     string `json:"allow-overwrite,omitempty"`
	// Never sent by the node; kept so Validate can reject it.
	IncrementalUpload string `json:"incremental-upload,omitempty"`
	ForcePathStyle    string `json:"force-path-style,omitempty"`
	// Passthrough flags, already deduplicated by the node.
	Args []string `json:"args,omitempty"`
}

func (args s3MounterArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
	result := []string{bucket, targetPath}

	for _, f := range []struct{ value, flag string }{
		{args.AllowOther, "--allow-other"},
		{args.ReadOnly, "--read-only"},
		{args.AllowDelete, "--allow-delete"},
		{args.AllowOverwrite, "--allow-overwrite"},
		{args.ForcePathStyle, "--force-path-style"},
	} {
		if isTrue(f.value) {
			result = append(result, f.flag)
		}
	}

	for _, f := range []struct{ value, flag string }{
		{args.UID, "--uid="},
		{args.GID, "--gid="},
		{args.EndpointURL, "--endpoint-url="},
		{args.Region, "--region="},
		{args.LogDirectory, "--log-directory="},
		{args.CacheDir, "--cache="},
	} {
		if f.value != "" {
			result = append(result, f.flag+f.value)
		}
	}

	if args.LogLevel != "" {
		result = append(result, "--"+args.LogLevel)
	}

	return append(result, args.Args...), nil
}

func (args s3MounterArgs) Validate(targetPath string) error {
	if err := pathValidator(targetPath); err != nil {
		return err
	}

	for _, f := range []struct{ name, value string }{
		{"allow-other", args.AllowOther},
		{"read-only", args.ReadOnly},
		{"allow-delete", args.AllowDelete},
		{"allow-overwrite", args.AllowOverwrite},
		{"incremental-upload", args.IncrementalUpload},
		{"force-path-style", args.ForcePathStyle},
	} {
		if err := validateBool(f.name, f.value); err != nil {
			return err
		}
	}

	if args.AwsCredentialsFile == "" {
		logger.Error("aws-credentials-file is required for mount-s3")
		return errors.New("aws-credentials-file is required for mount-s3")
	}
	if args.AwsConfigFile == "" {
		logger.Error("aws-config-file is required for mount-s3")
		return errors.New("aws-config-file is required for mount-s3")
	}

	// AWS-only (S3 Express One Zone); causes data loss on IBM COS.
	if isTrue(args.IncrementalUpload) {
		logger.Error("incremental-upload requires AWS S3 Express One Zone and causes data loss on IBM COS")
		return errors.New("incremental-upload is not supported: it requires AWS S3 Express One Zone and causes data loss on IBM COS")
	}

	if isTrue(args.ReadOnly) && (isTrue(args.AllowDelete) || isTrue(args.AllowOverwrite)) {
		logger.Error("read-only cannot be combined with allow-delete or allow-overwrite")
		return errors.New("read-only cannot be combined with allow-delete or allow-overwrite")
	}

	for _, f := range []struct{ name, value string }{
		{"uid", args.UID},
		{"gid", args.GID},
	} {
		if f.value == "" {
			continue
		}
		if _, err := strconv.Atoi(f.value); err != nil {
			logger.Error(f.name+" must be a valid integer", zap.String(f.name, f.value))
			return fmt.Errorf("%s must be a valid integer: %v", f.name, f.value)
		}
	}

	switch args.LogLevel {
	case "", "debug", "debug-crt", "no-log":
	default:
		logger.Error("unsupported log-level for mount-s3", zap.String("log-level", args.LogLevel))
		return fmt.Errorf("unsupported log-level %q for mount-s3, supported: debug, debug-crt, no-log", args.LogLevel)
	}

	if args.CacheDir != "" {
		if err := ensureDir(args.CacheDir, s3MounterSafeCacheDir); err != nil {
			logger.Error("failed to create cache directory", zap.String("cache-dir", args.CacheDir), zap.Error(err))
			return fmt.Errorf("failed to create cache directory '%s': %w", args.CacheDir, err)
		}
	}
	// mount-s3 creates the log directory itself.
	if args.LogDirectory != "" {
		if _, err := resolveUnderBase(args.LogDirectory, s3MounterSafeLogDir); err != nil {
			logger.Error("invalid log directory", zap.String("log-dir", args.LogDirectory), zap.Error(err))
			return fmt.Errorf("invalid log directory '%s': %w", args.LogDirectory, err)
		}
	}

	return nil
}
