package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// rcloneFlagPattern matches a valid rclone CLI flag token:
//
//	--flag-name          (boolean flag)
//	--flag-name=value    (flag with value; value is an opaque string)
//
// Flag names must start with a lowercase letter and contain only [a-z0-9-].
var rcloneFlagPattern = regexp.MustCompile(`^--([a-z][a-z0-9-]*)(?:=(.*))?$`)

// rcloneKnownFlags is the set of flag names already covered by the typed
// RCloneArgs fields. add-mount-param tokens whose flag name matches one of these
// are rejected to prevent accidental or malicious override of a validated
// parameter (e.g. trying to override --config via add-mount-param).
var rcloneKnownFlags = jsonFieldNames(RCloneArgs{})

var ReadConfigFile = os.ReadFile

// validateAddMountParam validates every comma-separated token in the add-mount-param
// string. Each token must look like --flag or --flag=value (lowercase rclone
// flag name format). Tokens whose flag name collides with a known RCloneArgs
// field are rejected to prevent override attacks.
func validateAddMountParam(extra string) error {
	for _, token := range strings.Split(extra, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		m := rcloneFlagPattern.FindStringSubmatch(token)
		if m == nil {
			return fmt.Errorf("add-mount-param: invalid token %q: must be --flag or --flag=value with a lowercase flag name", token)
		}
		flagName := m[1]
		if _, reserved := rcloneKnownFlags[flagName]; reserved {
			return fmt.Errorf("add-mount-param: flag %q is already a known parameter and cannot be set via add-mount-param", flagName)
		}
	}
	return nil
}

func validateRcloneConfigFile(configPath string) error {
	if configPath == "" {
		return nil
	}
	content, err := ReadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("cannot read rclone config file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	typeCount := 0
	sectionCount := 0

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionCount++
			section := strings.TrimSpace(line[1 : len(line)-1])
			if section != "ibmcos" {
				return fmt.Errorf("invalid section [%s] in rclone config at line %d; only [ibmcos] is permitted", section, lineNum+1)
			}
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.ToLower(strings.TrimSpace(parts[0]))
			v := strings.TrimSpace(parts[1])

			if k == "type" {
				typeCount++
				if strings.ToLower(v) != "s3" {
					return fmt.Errorf("invalid backend type %q in rclone config at line %d; only 's3' is permitted", v, lineNum+1)
				}
			}
		}
	}

	if sectionCount != 1 {
		return fmt.Errorf("expected exactly 1 section [ibmcos] in rclone config, found %d", sectionCount)
	}
	if typeCount != 1 {
		return fmt.Errorf("expected exactly 1 'type = s3' entry in rclone config, found %d", typeCount)
	}

	return nil
}

type RCloneArgs struct {
	AllowOther            string `json:"allow-other,omitempty"`
	AllowRoot             string `json:"allow-root,omitempty"`
	AsyncRead             string `json:"async-read,omitempty"`
	AttrTimeout           string `json:"attr-timeout,omitempty"`
	ConfigPath            string `json:"config,omitempty"`
	Daemon                string `json:"daemon,omitempty"`
	DaemonTimeout         string `json:"daemon-timeout,omitempty"`
	DaemonWait            string `json:"daemon-wait,omitempty"`
	DirCacheTime          string `json:"dir-cache-time,omitempty"`
	DirectIO              string `json:"direct-io,omitempty"`
	GID                   string `json:"gid,omitempty"`
	LogFile               string `json:"log-file,omitempty"`
	LogLevel              string `json:"log-level,omitempty"`
	MultiThreadCutoff     string `json:"multi-thread-cutoff,omitempty"`
	NoModificationTime    string `json:"no-modtime,omitempty"`
	PollInterval          string `json:"poll-interval,omitempty"`
	ReadOnly              string `json:"read-only,omitempty"`
	UID                   string `json:"uid,omitempty"`
	UMask                 string `json:"umask,omitempty"`
	VfsCacheMaxAge        string `json:"vfs-cache-max-age,omitempty"`
	VfsCacheMaxSize       string `json:"vfs-cache-max-size,omitempty"`
	VfsCacheMinFreeSpace  string `json:"vfs-cache-min-free-space,omitempty"`
	VfsCacheMode          string `json:"vfs-cache-mode,omitempty"`
	VfsCachePollInterval  string `json:"vfs-cache-poll-interval,omitempty"`
	VfsDiskSpaceTotalSize string `json:"vfs-disk-space-total-size,omitempty"`
	VfsReadAhead          string `json:"vfs-read-ahead,omitempty"`
	VfsReadChunkSize      string `json:"vfs-read-chunk-size,omitempty"`
	VfsReadChunkSizeLimit string `json:"vfs-read-chunk-size-limit,omitempty"`
	VfsReadChunkStreams   string `json:"vfs-read-chunk-streams,omitempty"`
	VfsReadWait           string `json:"vfs-read-wait,omitempty"`
	VfsRefresh            string `json:"vfs-refresh,omitempty"`
	VfsWriteBack          string `json:"vfs-write-back,omitempty"`
	VfsWriteWait          string `json:"vfs-write-wait,omitempty"`
	WriteBackCache        string `json:"write-back-cache,omitempty"`
	// AddMountParam allows passing additional rclone mount flags not covered by
	// the fields above, as a comma-separated list of --flag or --flag=value
	// tokens. Example: "--transfers=4,--checkers=8,--s3-upload-concurrency=16"
	AddMountParam string `json:"add-mount-param,omitempty"`
}

func (args RCloneArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
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

	// Convert to key=value slice, skipping add-mount-param (handled separately below)
	result := []string{"mount", bucket, targetPath}
	for k, v := range m {
		if k == "add-mount-param" {
			continue
		}
		result = append(result, fmt.Sprintf("--%s=%v", k, v)) // --key=value
	}

	// Append each validated add-mount-param token as its own argument
	if args.AddMountParam != "" {
		for _, token := range strings.Split(args.AddMountParam, ",") {
			token = strings.TrimSpace(token)
			if token != "" {
				result = append(result, token)
			}
		}
	}

	return result, nil // [mount, bucket, path, --key1=value1, --key2=value2, ..., --extra1=val, ...]
}

func (args RCloneArgs) Validate(targetPath string) error {
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

	// Check if value of allow-root parameter is boolean "true" or "false"
	if args.AllowRoot != "" {
		if isBool := isBoolString(args.AllowRoot); !isBool {
			logger.Error("cannot convert value of allow-root into boolean", zap.Any("allow-root", args.AllowRoot))
			return fmt.Errorf("cannot convert value of allow-root into boolean: %v", args.AllowRoot)
		}
	}

	// Check if value of async-read parameter is boolean "true" or "false"
	if args.AsyncRead != "" {
		if isBool := isBoolString(args.AsyncRead); !isBool {
			logger.Error("cannot convert value of async-read into boolean", zap.Any("async-read", args.AsyncRead))
			return fmt.Errorf("cannot convert value of async-read into boolean: %v", args.AsyncRead)
		}
	}

	// Check if rclone config file exists or not
	if exists, err := FileExists(args.ConfigPath); err != nil {
		logger.Error("error checking rclone config file existence")
		return fmt.Errorf("error checking rclone config file existence")
	} else if !exists {
		logger.Error("rclone config file not found")
		return fmt.Errorf("rclone config file not found")
	}

	if err := validateRcloneConfigFile(args.ConfigPath); err != nil {
		logger.Error("rclone config file validation failed", zap.Error(err))
		return fmt.Errorf("rclone config validation failed: %w", err)
	}

	// Check if value of daemon parameter is boolean "true" or "false"
	if args.Daemon != "" {
		if isBool := isBoolString(args.Daemon); !isBool {
			logger.Error("cannot convert value of daemon into boolean", zap.Any("daemon", args.Daemon))
			return fmt.Errorf("cannot convert value of daemon into boolean: %v", args.Daemon)
		}
	}

	// Check if value of direct-io parameter is boolean "true" or "false"
	if args.DirectIO != "" {
		if isBool := isBoolString(args.DirectIO); !isBool {
			logger.Error("cannot convert value of direct-io into boolean", zap.Any("direct-io", args.DirectIO))
			return fmt.Errorf("cannot convert value of direct-io into boolean: %v", args.DirectIO)
		}
	}

	// Check if value of no-modtime parameter is boolean "true" or "false"
	if args.NoModificationTime != "" {
		if isBool := isBoolString(args.NoModificationTime); !isBool {
			logger.Error("cannot convert value of no-modtime into boolean", zap.Any("no-modtime", args.NoModificationTime))
			return fmt.Errorf("cannot convert value of no-modtime into boolean: %v", args.NoModificationTime)
		}
	}

	// Check if value of read-only parameter is boolean "true" or "false"
	if args.ReadOnly != "" {
		if isBool := isBoolString(args.ReadOnly); !isBool {
			logger.Error("cannot convert value of read-only into boolean", zap.Any("read-only", args.ReadOnly))
			return fmt.Errorf("cannot convert value of read-only into boolean: %v", args.ReadOnly)
		}
	}

	// Check if value of vfs-refresh parameter is boolean "true" or "false"
	if args.VfsRefresh != "" {
		if isBool := isBoolString(args.VfsRefresh); !isBool {
			logger.Error("cannot convert value of vfs-refresh into boolean", zap.Any("vfs-refresh", args.VfsRefresh))
			return fmt.Errorf("cannot convert value of vfs-refresh into boolean: %v", args.VfsRefresh)
		}
	}

	// Check if value of write-back-cache parameter is boolean "true" or "false"
	if args.WriteBackCache != "" {
		if isBool := isBoolString(args.WriteBackCache); !isBool {
			logger.Error("cannot convert value of write-back-cache into boolean", zap.Any("write-back-cache", args.WriteBackCache))
			return fmt.Errorf("cannot convert value of write-back-cache into boolean: %v", args.WriteBackCache)
		}
	}

	// Validate add-mount-param tokens: format and reserved-name check
	if args.AddMountParam != "" {
		if err := validateAddMountParam(args.AddMountParam); err != nil {
			logger.Error("invalid add-mount-param value", zap.Error(err))
			return err
		}
	}

	return nil
}
