package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type S3FSArgs struct {
	AllowOther              string `json:"allow_other,omitempty"`
	AutoCache               string `json:"auto_cache,omitempty"`
	CipherSuites            string `json:"cipher_suites,omitempty"`
	ConnectTimeoutSeconds   string `json:"connect_timeout,omitempty"`
	CurlDebug               string `json:"curldbg,omitempty"`
	DebugLevel              string `json:"dbglevel,omitempty"`
	DefaultACL              string `json:"default_acl,omitempty"`
	EndPoint                string `json:"endpoint,omitempty"`
	GID                     string `json:"gid,omitempty"`
	IBMIamAuth              string `json:"ibm_iam_auth,omitempty"`
	IBMIamEndpoint          string `json:"ibm_iam_endpoint,omitempty"`
	InstanceName            string `json:"instance_name,omitempty"`
	KernelCache             string `json:"kernel_cache,omitempty"`
	MaxBackground           string `json:"max_background,omitempty"`
	MaxStatCacheSize        string `json:"max_stat_cache_size,omitempty"`
	MPUmask                 string `json:"mp_umask,omitempty"`
	MultipartSize           string `json:"multipart_size,omitempty"`
	MultiReqMax             string `json:"multireq_max,omitempty"`
	ParallelCount           string `json:"parallel_count,omitempty"`
	PasswdFilePath          string `json:"passwd_file,omitempty"`
	ReadOnly                string `json:"ro,omitempty"`
	ReadwriteTimeoutSeconds string `json:"readwrite_timeout,omitempty"`
	RetryCount              string `json:"retries,omitempty"`
	SigV2                   string `json:"sigv2,omitempty"`
	SigV4                   string `json:"sigv4,omitempty"`
	StatCacheExpireSeconds  string `json:"stat_cache_expire,omitempty"`
	UID                     string `json:"uid,omitempty"`
	URL                     string `json:"url,omitempty"`
	UsePathRequestStyle     string `json:"use_path_request_style,omitempty"`
	UseXattr                string `json:"use_xattr,string,omitempty"`
}

func (args S3FSArgs) PopulateArgsSlice(bucket, targetPath string) ([]string, error) {
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
	result := []string{bucket, targetPath}
	for k, v := range m {
		result = append(result, "-o")
		if strings.ToLower(strings.TrimSpace(v)) == "true" {
			result = append(result, fmt.Sprintf("%s", k)) // -o, key
		} else {
			result = append(result, fmt.Sprintf("%s=%v", k, v)) // -o, key=value
		}
	}

	return result, nil // [bucket, path, -o, key1=value1, -o, key2=value2, -o key3, ...]
}

func (args S3FSArgs) Validate(targetPath string) error {
	if err := pathValidator(targetPath); err != nil {
		return err
	}

	retryCount, err := strconv.Atoi(args.RetryCount)
	if err != nil {
		logger.Error("cannot convert value of retires into integer", zap.Error(err))
		return fmt.Errorf("Cannot convert value of retires into integer: %v", err)
	}
	if retryCount < 1 {
		logger.Error("value of retires should be >= 1", zap.Error(err))
		return fmt.Errorf("value of retires should be >= 1")
	}
	return nil
}
