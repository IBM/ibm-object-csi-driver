package main

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

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
	InvalidValue          string `json:"invalid-value,omitempty"`
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

	// Convert to key=value slice
	result := []string{"mount", bucket, targetPath}
	for k, v := range m {
		result = append(result, fmt.Sprintf("--%s=%v", k, v)) // --key=value
	}

	return result, nil // [mount, bucket, path, --key1=value1, --key2=value2, ...]
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

	return nil
}
