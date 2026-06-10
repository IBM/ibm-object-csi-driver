//go:build linux
// +build linux

/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package mounter
package mounter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/logger"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"go.uber.org/zap"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type S3fsMounter struct {
	BucketName    string //From Secret in SC
	ObjectPath    string //From Secret in SC
	EndPoint      string //From Secret in SC
	LocConstraint string //From Secret in SC
	AuthType      string
	AccessKeys    string
	IAMEndpoint   string
	KpRootKeyCrn  string
	MountOptions  []string
	MounterUtils  utils.MounterUtils
	logger        *zap.Logger
}

const (
	passFile   = ".passwd-s3fs" // #nosec G101: not password
	maxRetries = 3
)

var (
	writePassWrap = writePass
	removeFile    = removeS3FSCredFile
	// s3fsLogger is used for package-level logging where context is not available
	s3fsLogger *zap.Logger
)

func init() {
	var err error
	s3fsLogger, err = zap.NewProduction()
	if err != nil {
		// Fallback to no-op logger if production logger fails
		s3fsLogger = zap.NewNop()
	}
}

func NewS3fsMounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils, defaultParams map[string]string) Mounter {
	var (
		val       string
		check     bool
		accessKey string
		secretKey string
		apiKey    string
		mounter   *S3fsMounter
	)

	mounter = &S3fsMounter{}

	if val, check = secretMap["cosEndpoint"]; check {
		mounter.EndPoint = val
	}
	if val, check = secretMap["locationConstraint"]; check {
		mounter.LocConstraint = val
	}
	if val, check = secretMap["bucketName"]; check {
		mounter.BucketName = val
	}
	if val, check = secretMap["objectPath"]; check {
		mounter.ObjectPath = val
	}
	if val, check = secretMap["accessKey"]; check {
		accessKey = val
	}
	if val, check = secretMap["secretKey"]; check {
		secretKey = val
	}
	if val, check = secretMap["apiKey"]; check {
		apiKey = val
	}
	if val, check = secretMap["kpRootKeyCRN"]; check {
		mounter.KpRootKeyCrn = val
	}
	if val, check = secretMap["iamEndpoint"]; check {
		mounter.IAMEndpoint = val
	}

	if apiKey != "" {
		mounter.AccessKeys = fmt.Sprintf(":%s", apiKey)
		mounter.AuthType = "iam"
	} else {
		mounter.AccessKeys = fmt.Sprintf("%s:%s", accessKey, secretKey)
		mounter.AuthType = "hmac"
	}

	updatedOptions := updateS3FSMountOptions(mountOptions, secretMap, defaultParams)
	mounter.MountOptions = updatedOptions

	mounter.MounterUtils = mounterUtils

	// Initialize logger - use production logger or fallback to nop
	mounter.logger, _ = zap.NewProduction()
	if mounter.logger == nil {
		mounter.logger = zap.NewNop()
	}

	return mounter
}

func (s3fs *S3fsMounter) Mount(ctx context.Context, source string, target string) error {
	logger.Info(ctx, s3fs.logger, "S3FSMounter Mount started",
		zap.String("source", source), zap.String("target", target))

	reqID := requestid.FromContext(ctx) // Still needed for error messages

	var s3fsCredDir string
	if mountWorker {
		s3fsCredDir = constants.MounterConfigPathOnHost
	} else {
		s3fsCredDir = constants.MounterConfigPathOnPodS3fs
	}

	var bucketName string
	var pathExist bool
	var err error

	metaPath := path.Join(s3fsCredDir, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if pathExist, err = checkPath(metaPath); err != nil {
		logger.Error(ctx, s3fs.logger, "Cannot stat directory", zap.String("meta_path", metaPath), zap.Error(err))
		return fmt.Errorf("[%s] S3FSMounter Mount: Cannot stat directory %s: %v", reqID, metaPath, err)
	}

	if !pathExist {
		logger.Debug(ctx, s3fs.logger, "Creating meta directory", zap.String("meta_path", metaPath))
		if err = MakeDir(metaPath, 0755); // #nosec G301: used for s3fs
		err != nil {
			logger.Error(ctx, s3fs.logger, "Cannot create directory", zap.String("meta_path", metaPath), zap.Error(err))
			return fmt.Errorf("[%s] S3FSMounter Mount: Cannot create directory %s: %v", reqID, metaPath, err)
		}
	}

	passwdFile := path.Join(metaPath, passFile)
	if err = writePassWrap(passwdFile, s3fs.AccessKeys); err != nil {
		logger.Error(ctx, s3fs.logger, "Cannot create password file", zap.String("passwd_file", passwdFile), zap.Error(err))
		return fmt.Errorf("[%s] S3FSMounter Mount: Cannot create file %s: %v", reqID, passwdFile, err)
	}

	if s3fs.ObjectPath != "" {
		if strings.HasPrefix(s3fs.ObjectPath, "/") {
			bucketName = fmt.Sprintf("%s:%s", s3fs.BucketName, s3fs.ObjectPath)
		} else {
			bucketName = fmt.Sprintf("%s:/%s", s3fs.BucketName, s3fs.ObjectPath)
		}
	} else {
		bucketName = s3fs.BucketName
	}

	logger.Info(ctx, s3fs.logger, "Formulating mount options",
		zap.String("bucket_name", bucketName),
		zap.String("auth_type", s3fs.AuthType))
	args, wnOp := s3fs.formulateMountOptions(bucketName, target, passwdFile)

	if mountWorker {
		logger.Info(ctx, s3fs.logger, "Mount on Worker started")

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			logger.Error(ctx, s3fs.logger, "Error marshalling data", zap.Error(err))
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`, target, bucketName, constants.S3FS, jsonData)

		logger.Debug(ctx, s3fs.logger, "Worker mounting payload", zap.String("payload", payload))

		err = mounterRequest(ctx, payload, "http://unix/api/cos/mount", s3fs.logger)
		if err != nil {
			logger.Error(ctx, s3fs.logger, "Failed to mount on worker", zap.Error(err))
			return err
		}
		logger.Info(ctx, s3fs.logger, "S3FSMounter Mount completed successfully on worker")
		return nil
	}

	logger.Info(ctx, s3fs.logger, "NodeServer mounting")
	err = s3fs.MounterUtils.FuseMount(target, constants.S3FS, args)
	if err != nil {
		logger.Error(ctx, s3fs.logger, "FuseMount failed", zap.Error(err))
	} else {
		logger.Info(ctx, s3fs.logger, "S3FSMounter Mount completed successfully")
	}
	return err
}

func (s3fs *S3fsMounter) Unmount(ctx context.Context, target string) error {
	logger.Info(ctx, s3fs.logger, "S3FSMounter Unmount started", zap.String("target", target))

	if mountWorker {
		logger.Info(ctx, s3fs.logger, "Unmount on Worker started")

		payload := fmt.Sprintf(`{"path":"%s"}`, target)

		err := mounterRequest(ctx, payload, "http://unix/api/cos/unmount", s3fs.logger)
		if err != nil {
			logger.Error(ctx, s3fs.logger, "Failed to unmount on worker", zap.Error(err))
			return err
		}

		removeFile(constants.MounterConfigPathOnHost, target)
		logger.Info(ctx, s3fs.logger, "S3FSMounter Unmount completed successfully on worker")
		return nil
	}

	logger.Info(ctx, s3fs.logger, "NodeServer unmounting")

	err := s3fs.MounterUtils.FuseUnmount(target)
	if err != nil {
		logger.Error(ctx, s3fs.logger, "FuseUnmount failed", zap.Error(err))
		return err
	}

	removeFile(constants.MounterConfigPathOnPodS3fs, target)
	logger.Info(ctx, s3fs.logger, "S3FSMounter Unmount completed successfully")
	return nil
}

func updateS3FSMountOptions(defaultMountOp []string, secretMap map[string]string, defaultParams map[string]string) []string {
	mountOptsMap := make(map[string]string)

	// Create map out of array
	for _, val := range defaultMountOp {
		if strings.TrimSpace(val) == "" {
			continue
		}
		opts := strings.Split(val, "=")
		if len(opts) == 2 {
			mountOptsMap[opts[0]] = opts[1]
		} else if len(opts) == 1 {
			mountOptsMap[opts[0]] = opts[0]
		}
	}

	if val, check := secretMap["tmpdir"]; check {
		mountOptsMap["tmpdir"] = val
	}

	if val, check := secretMap["use_cache"]; check {
		mountOptsMap["use_cache"] = val
	}

	if val, check := secretMap["gid"]; check {
		mountOptsMap["gid"] = val
	}

	if secretMap["gid"] != "" && secretMap["uid"] == "" {
		mountOptsMap["uid"] = secretMap["gid"]
	} else if secretMap["uid"] != "" {
		mountOptsMap["uid"] = secretMap["uid"]
	}

	stringData, ok := secretMap["mountOptions"]
	if !ok {
		s3fsLogger.Info("No new mountOptions found. Using default mountOptions", zap.Any("default_mount_options", mountOptsMap))
	} else {
		lines := strings.Split(stringData, "\n")
		// Update map
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			opts := strings.Split(line, "=")
			if len(opts) == 2 {
				mountOptsMap[strings.TrimSpace(opts[0])] = strings.TrimSpace(opts[1])
			} else if len(opts) == 1 {
				mountOptsMap[strings.TrimSpace(opts[0])] = strings.TrimSpace(opts[0])
			} else {
				s3fsLogger.Info("Invalid mount option", zap.String("line", line))
			}
		}
	}

	// Create array out of map
	updatedOptions := []string{}
	for key, val := range mountOptsMap {
		option := fmt.Sprintf("%s=%s", key, val)
		isKeyValuePair := true

		if key == val {
			isKeyValuePair = false
			option = val
		}

		if newVal, check := secretMap[key]; check {
			if isKeyValuePair {
				option = fmt.Sprintf("%s=%s", key, newVal)
			} else {
				option = newVal
			}
		}

		updatedOptions = append(updatedOptions, option)
	}

	// Mount options which are not present in secret mountOptions and need to be set by nodeserver
	for key, value := range defaultParams {
		if value != "" {
			if _, ok := mountOptsMap[key]; !ok {
				option := fmt.Sprintf("%s=%s", key, value)
				updatedOptions = append(updatedOptions, option)
			}
		}
	}

	s3fsLogger.Info("updated S3fsMounter Options", zap.Any("updated_options", updatedOptions))
	return updatedOptions
}

func (s3fs *S3fsMounter) formulateMountOptions(bucket, target, passwdFile string) (nodeServerOp []string, workerNodeOp map[string]string) {
	nodeServerOp = []string{
		bucket,
		target,
		"-o", "sigv2",
		"-o", "use_path_request_style",
		"-o", fmt.Sprintf("passwd_file=%s", passwdFile),
		"-o", fmt.Sprintf("url=%s", s3fs.EndPoint),
		"-o", "allow_other",
		"-o", "mp_umask=002",
	}

	workerNodeOp = map[string]string{
		"sigv2":                  "true",
		"use_path_request_style": "true",
		"passwd_file":            passwdFile,
		"url":                    s3fs.EndPoint,
		"allow_other":            "true",
		"mp_umask":               "002",
	}

	if s3fs.LocConstraint != "" {
		nodeServerOp = append(nodeServerOp, "-o", fmt.Sprintf("endpoint=%s", s3fs.LocConstraint))
		workerNodeOp["endpoint"] = s3fs.LocConstraint
	}

	for _, val := range s3fs.MountOptions {
		nodeServerOp = append(nodeServerOp, "-o")
		nodeServerOp = append(nodeServerOp, val)

		splitVal := strings.Split(val, "=")
		if len(splitVal) == 1 {
			workerNodeOp[splitVal[0]] = "true"
		} else {
			workerNodeOp[splitVal[0]] = splitVal[1]
		}
	}

	if s3fs.AuthType != "hmac" {
		nodeServerOp = append(nodeServerOp, "-o", "ibm_iam_auth")
		nodeServerOp = append(nodeServerOp, "-o", "ibm_iam_endpoint="+s3fs.IAMEndpoint)

		workerNodeOp["ibm_iam_auth"] = "true"
		workerNodeOp["ibm_iam_endpoint"] = s3fs.IAMEndpoint
	} else {
		nodeServerOp = append(nodeServerOp, "-o", "default_acl=private")
		workerNodeOp["default_acl"] = "private"
	}
	return
}

func removeS3FSCredFile(credDir, target string) {
	metaPath := path.Join(credDir, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	for retry := 1; retry <= maxRetries; retry++ {
		_, err := Stat(metaPath)
		if err != nil {
			if os.IsNotExist(err) {
				s3fsLogger.Info("removeS3FSCredFile: Password file directory does not exist", zap.String("path", metaPath))
				return
			}
			s3fsLogger.Error("removeS3FSCredFile: Failed to stat path", zap.Int("attempt", retry), zap.String("path", metaPath), zap.Error(err))
			time.Sleep(constants.Interval)
			continue
		}
		err = RemoveAll(metaPath)
		if err != nil {
			s3fsLogger.Error("removeS3FSCredFile: Failed to remove password file path", zap.Int("attempt", retry), zap.String("path", metaPath), zap.Error(err))
			time.Sleep(constants.Interval)
			continue
		}
		s3fsLogger.Info("removeS3FSCredFile: Successfully removed password file path", zap.String("path", metaPath))
		return
	}
	s3fsLogger.Error("removeS3FSCredFile: Failed to remove password file after max attempts", zap.Int("max_attempts", maxRetries))
}
