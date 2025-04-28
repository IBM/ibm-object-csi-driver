package main

import (
	"encoding/json"
	"fmt"
)

type RCloneArgs struct {
	AllowOther            string `json:"allow-other,omitempty"`
	AllowRoot             string `json:"allow-root,omitempty"`
	AsyncRead             string `json:"async-read,omitempty"`
	AttrTimeout           string `josn:"attr-timeout,omitempty"`
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
	return nil
}
