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
}

const (
	passFile   = ".passwd-s3fs" // #nosec G101: not password
	maxRetries = 3
)

var (
	writePassWrap   = writePass
	removeFile      = removeS3FSCredFile
	s3fsLogger, _   = zap.NewProduction()
)

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

	return mounter
}

func (s3fs *S3fsMounter) Mount(ctx context.Context, source string, target string) error {
	reqID := requestid.FromContext(ctx)
	baseLogger, _ := zap.NewProduction()
	log := logger.WithRequestID(ctx, baseLogger)
	
	log.Info(fmt.Sprintf("[%s] S3FSMounter Mount started", reqID),
		zap.String("source", source), zap.String("target", target))

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
		log.Error(fmt.Sprintf("[%s] Cannot stat directory", reqID), zap.String("meta_path", metaPath), zap.Error(err))
		return fmt.Errorf("[%s] S3FSMounter Mount: Cannot stat directory %s: %v", reqID, metaPath, err)
	}

	if !pathExist {
		log.Debug(fmt.Sprintf("[%s] Creating meta directory", reqID), zap.String("meta_path", metaPath))
		if err = MakeDir(metaPath, 0755); // #nosec G301: used for s3fs
		err != nil {
			log.Error(fmt.Sprintf("[%s] Cannot create directory", reqID), zap.String("meta_path", metaPath), zap.Error(err))
			return fmt.Errorf("[%s] S3FSMounter Mount: Cannot create directory %s: %v", reqID, metaPath, err)
		}
	}

	passwdFile := path.Join(metaPath, passFile)
	if err = writePassWrap(passwdFile, s3fs.AccessKeys); err != nil {
		log.Error(fmt.Sprintf("[%s] Cannot create password file", reqID), zap.String("passwd_file", passwdFile), zap.Error(err))
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

	log.Info(fmt.Sprintf("[%s] Formulating mount options", reqID),
		zap.String("bucket_name", bucketName),
		zap.String("auth_type", s3fs.AuthType))
	args, wnOp := s3fs.formulateMountOptions(bucketName, target, passwdFile)

	if mountWorker {
		log.Info(fmt.Sprintf("[%s] Mount on Worker started", reqID))

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Error marshalling data", reqID), zap.Error(err))
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`, target, bucketName, constants.S3FS, jsonData)

		log.Debug(fmt.Sprintf("[%s] Worker mounting payload", reqID), zap.String("payload", payload))

		err = mounterRequest(ctx, payload, "http://unix/api/cos/mount", log)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Failed to mount on worker", reqID), zap.Error(err))
			return err
		}
		log.Info(fmt.Sprintf("[%s] S3FSMounter Mount completed successfully on worker", reqID))
		return nil
	}
	
	log.Info(fmt.Sprintf("[%s] NodeServer mounting", reqID))
	err = s3fs.MounterUtils.FuseMount(target, constants.S3FS, args)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] FuseMount failed", reqID), zap.Error(err))
	} else {
		log.Info(fmt.Sprintf("[%s] S3FSMounter Mount completed successfully", reqID))
	}
	return err
}

func (s3fs *S3fsMounter) Unmount(ctx context.Context, target string) error {
	reqID := requestid.FromContext(ctx)
	baseLogger, _ := zap.NewProduction()
	log := logger.WithRequestID(ctx, baseLogger)
	
	log.Info(fmt.Sprintf("[%s] S3FSMounter Unmount started", reqID), zap.String("target", target))

	if mountWorker {
		log.Info(fmt.Sprintf("[%s] Unmount on Worker started", reqID))

		payload := fmt.Sprintf(`{"path":"%s"}`, target)

		err := mounterRequest(ctx, payload, "http://unix/api/cos/unmount", log)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Failed to unmount on worker", reqID), zap.Error(err))
			return err
		}

		removeFile(constants.MounterConfigPathOnHost, target)
		log.Info(fmt.Sprintf("[%s] S3FSMounter Unmount completed successfully on worker", reqID))
		return nil
	}
	
	log.Info(fmt.Sprintf("[%s] NodeServer unmounting", reqID))

	err := s3fs.MounterUtils.FuseUnmount(target)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] FuseUnmount failed", reqID), zap.Error(err))
		return err
	}

	removeFile(constants.MounterConfigPathOnPodS3fs, target)
	log.Info(fmt.Sprintf("[%s] S3FSMounter Unmount completed successfully", reqID))
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
