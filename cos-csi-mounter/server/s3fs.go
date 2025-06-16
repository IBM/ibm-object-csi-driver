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
	MaxDirtyData            string `json:"max_dirty_data,omitempty"`
	MaxStatCacheSize        string `json:"max_stat_cache_size,omitempty"`
	MPUmask                 string `json:"mp_umask,omitempty"`
	MultiPartSize           string `json:"multipart_size,omitempty"`
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
	Umask                   string `json:"umask,omitempty"`
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
			result = append(result, k) // -o, key
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

	// Check if value of allow_other parameter is boolean "true" or "false"
	if args.AllowOther != "" {
		if isBool := isBoolString(args.AllowOther); !isBool {
			logger.Error("cannot convert value of allow_other into boolean", zap.Any("allow_other", args.AllowOther))
			return fmt.Errorf("cannot convert value of allow_other into boolean: %v", args.AllowOther)
		}
	}

	// Check if value of auto_cache parameter is boolean "true" or "false"
	if args.AutoCache != "" {
		if isBool := isBoolString(args.AutoCache); !isBool {
			logger.Error("cannot convert value of auto_cache into boolean", zap.Any("auto_cache", args.AutoCache))
			return fmt.Errorf("cannot convert value of auto_cache into boolean: %v", args.AutoCache)
		}
	}

	// Check if value of connect_timeout parameter can be converted to integer
	if args.ConnectTimeoutSeconds != "" {
		_, err := strconv.Atoi(args.ConnectTimeoutSeconds)
		if err != nil {
			logger.Error("cannot convert value of connect_timeout into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of connect_timeout into integer: %v", err)
		}
	}

	// Check if value of curldbg parameter is either "body" or "normal"
	if args.CurlDebug != "" && args.CurlDebug != "body" && args.CurlDebug != "normal" {
		logger.Error("invalid value for 'curldbg' param. Should be either 'body' or 'normal'", zap.Any("curldbg", args.CurlDebug))
		return fmt.Errorf("invalid value for 'curldbg' param. Should be either 'body' or 'normal': %v", args.CurlDebug)
	}

	// Check if value of gid parameter can be converted to integer
	if args.GID != "" {
		_, err := strconv.Atoi(args.GID)
		if err != nil {
			logger.Error("cannot convert value of gid into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of gid into integer: %v", err)
		}
	}

	// Check if value of ibm_iam_auth parameter is boolean "true" or "false"
	if args.IBMIamAuth != "" {
		if isBool := isBoolString(args.IBMIamAuth); !isBool {
			logger.Error("cannot convert value of ibm_iam_auth into boolean", zap.Any("ibm_iam_auth", args.IBMIamAuth))
			return fmt.Errorf("cannot convert value of ibm_iam_auth into boolean: %v", args.IBMIamAuth)
		}
	}

	if args.IBMIamEndpoint != "" {
		if !strings.HasPrefix(args.IBMIamEndpoint, "https://") && !strings.HasPrefix(args.IBMIamEndpoint, "http://") {
			logger.Error("bad value for ibm_iam_endpoint."+
				" Must be of the form https://<hostname> or http://<hostname>",
				zap.String("ibm_iam_endpoint", args.IBMIamEndpoint))
			return fmt.Errorf("bad value for ibm_iam_endpoint \"%v\":"+
				" Must be of the form https://<hostname> or http://<hostname>", args.IBMIamEndpoint)
		}
	}

	// Check if value of kernel_cache parameter is boolean "true" or "false"
	if args.KernelCache != "" {
		if isBool := isBoolString(args.KernelCache); !isBool {
			logger.Error("cannot convert value of kernel_cache into boolean", zap.Any("kernel_cache", args.KernelCache))
			return fmt.Errorf("cannot convert value of kernel_cache into boolean: %v", args.KernelCache)
		}
	}

	// Check if value of max_background parameter can be converted to integer
	if args.MaxBackground != "" {
		_, err := strconv.Atoi(args.MaxBackground)
		if err != nil {
			logger.Error("cannot convert value of max_background into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of max_background into integer: %v", err)
		}
	}

	// Check if value of max_dirty_data parameter can be converted to integer
	if args.MaxDirtyData != "" {
		_, err := strconv.Atoi(args.MaxDirtyData)
		if err != nil {
			logger.Error("cannot convert value of max_dirty_data into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of max_dirty_data into integer: %v", err)
		}
	}

	// Check if value of max_stat_cache_size parameter can be converted to integer
	if args.MaxStatCacheSize != "" {
		_, err := strconv.Atoi(args.MaxStatCacheSize)
		if err != nil {
			logger.Error("cannot convert value of max_stat_cache_size into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of max_stat_cache_size into integer: %v", err)
		}
	}

	// Check if value of multipart_size parameter can be converted to integer
	if args.MultiPartSize != "" {
		_, err := strconv.Atoi(args.MultiPartSize)
		if err != nil {
			logger.Error("cannot convert value of multipart_size into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of multipart_size into integer: %v", err)
		}
	}

	// Check if value of multireq_max parameter can be converted to integer
	if args.MultiReqMax != "" {
		_, err := strconv.Atoi(args.MultiReqMax)
		if err != nil {
			logger.Error("cannot convert value of multireq_max into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of multireq_max into integer: %v", err)
		}
	}

	// Check if value of parallel_count parameter can be converted to integer
	if args.ParallelCount != "" {
		_, err := strconv.Atoi(args.ParallelCount)
		if err != nil {
			logger.Error("cannot convert value of parallel_count into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of parallel_count into integer: %v", err)
		}
	}

	// Check if .passwd file exists or not
	if exists, err := FileExists(args.PasswdFilePath); err != nil {
		logger.Error("error checking credential file existence")
		return fmt.Errorf("error checking credential file existence")
	} else if !exists {
		logger.Error("credential file not found")
		return fmt.Errorf("credential file not found")
	}

	// Check if value of ro parameter is boolean "true" or "false"
	if args.ReadOnly != "" {
		if isBool := isBoolString(args.ReadOnly); !isBool {
			logger.Error("cannot convert value of ro into boolean", zap.Any("ro", args.ReadOnly))
			return fmt.Errorf("cannot convert value of roe into boolean: %v", args.ReadOnly)
		}
	}

	// Check if value of readwrite_timeout parameter can be converted to integer
	if args.ReadwriteTimeoutSeconds != "" {
		_, err := strconv.Atoi(args.ReadwriteTimeoutSeconds)
		if err != nil {
			logger.Error("cannot convert value of readwrite_timeout into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of readwrite_timeout into integer: %v", err)
		}
	}

	// Check if value of retries parameter can be converted to integer
	if args.RetryCount != "" {
		retryCount, err := strconv.Atoi(args.RetryCount)
		if err != nil {
			logger.Error("cannot convert value of retires into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of retires into integer: %v", err)
		}
		if retryCount < 1 {
			logger.Error("value of retires should be >= 1")
			return fmt.Errorf("value of retires should be >= 1")
		}
	}

	// Check if value of sigv2 parameter is boolean "true" or "false"
	if args.SigV2 != "" {
		if isBool := isBoolString(args.SigV2); !isBool {
			logger.Error("cannot convert value of sigv2 into boolean", zap.Any("sigv2", args.SigV2))
			return fmt.Errorf("cannot convert value of sigv2 into boolean: %v", args.SigV2)
		}
	}

	// Check if value of sigv4 parameter is boolean "true" or "false"
	if args.SigV4 != "" {
		if isBool := isBoolString(args.SigV4); !isBool {
			logger.Error("cannot convert value of sigv4 into boolean", zap.Any("sigv4", args.SigV4))
			return fmt.Errorf("cannot convert value of sigv4 into boolean: %v", args.SigV4)
		}
	}

	// Check if value of stat_cache_expire parameter can be converted to integer
	if args.StatCacheExpireSeconds != "" {
		cacheExpireSeconds, err := strconv.Atoi(args.StatCacheExpireSeconds)
		if err != nil {
			logger.Error("cannot convert value of stat_cache_expire into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of stat_cache_expire into integer: %v", err)
		} else if cacheExpireSeconds < 0 {
			logger.Error("value of stat_cache_expire should be >= 0")
			return fmt.Errorf("value ofstat_cache_expire should be >= 0")
		}
	}

	// Check if value of uid parameter can be converted to integer
	if args.UID != "" {
		_, err := strconv.Atoi(args.UID)
		if err != nil {
			logger.Error("cannot convert value of uid into integer", zap.Error(err))
			return fmt.Errorf("cannot convert value of uid into integer: %v", err)
		}
	}

	// Check if value of use_path_request_style parameter is boolean "true" or "false"
	if args.UsePathRequestStyle != "" {
		if isBool := isBoolString(args.UsePathRequestStyle); !isBool {
			logger.Error("cannot convert value of use_path_request_style into boolean", zap.Any("use_path_request_style", args.UsePathRequestStyle))
			return fmt.Errorf("cannot convert value of use_path_request_style into boolean: %v", args.UsePathRequestStyle)
		}
	}

	// Check if value of use_xattr parameter is boolean "true" or "false"
	if args.UseXattr != "" {
		if isBool := isBoolString(args.UseXattr); !isBool {
			logger.Error("cannot convert value of use_xattr into boolean", zap.Any("use_xattr", args.UseXattr))
			return fmt.Errorf("cannot convert value of use_xattr into boolean: %v", args.UseXattr)
		}
	}

	if !strings.HasPrefix(args.URL, "https://") && !strings.HasPrefix(args.URL, "http://") {
		logger.Error("bad value for url: scheme is missing."+
			" Must be of the form http://<hostname> or https://<hostname>",
			zap.String("url", args.URL))
		return fmt.Errorf("bad value for url \"%v\": scheme is missing."+
			" Must be of the form http://<hostname> or https://<hostname>", args.URL)
	}

	return nil
}
