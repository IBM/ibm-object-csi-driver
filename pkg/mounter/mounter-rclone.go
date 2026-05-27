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
	"bufio"
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
// rcloneMounter Implements Mounter
type RcloneMounter struct {
	BucketName        string //From Secret in SC
	ObjectPath        string //From Secret in SC
	EndPoint          string //From Secret in SC
	LocConstraint     string //From Secret in SC
	AuthType          string
	AccessKeys        string
	serviceInstanceID string
	KpRootKeyCrn      string
	IAMEndpoint       string
	UID               string
	GID               string
	MountOptions      []string
	MounterUtils      utils.MounterUtils
}

const (
	configFileName = "rclone.conf"
	remote         = "ibmcos"
	s3Type         = "s3"
	cosProvider    = "IBMCOS"
)

var (
	createConfigWrap = createConfig
	removeConfigFile = removeRcloneConfigFile
	rcloneLogger, _  = zap.NewProduction()
)

func NewRcloneMounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
	var (
		val       string
		check     bool
		accessKey string
		secretKey string
		apiKey    string
		serviceId string
		mounter   *RcloneMounter
	)

	mounter = &RcloneMounter{}

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
	if val, check = secretMap["kpRootKeyCRN"]; check {
		mounter.KpRootKeyCrn = val
	}
	if val, check = secretMap["iamEndpoint"]; check {
		mounter.IAMEndpoint = val
	}
	if val, check = secretMap["apiKey"]; check {
		apiKey = val
	}
	if val, check = secretMap["serviceId"]; check {
		serviceId = val
	}

	if apiKey != "" {
		mounter.AccessKeys = apiKey
		mounter.serviceInstanceID = serviceId
		mounter.AuthType = "iam"
	} else {
		mounter.AccessKeys = fmt.Sprintf("%s:%s", accessKey, secretKey)
		mounter.AuthType = "hmac"
	}

	if val, check = secretMap["gid"]; check {
		mounter.GID = val
	}
	if secretMap["gid"] != "" && secretMap["uid"] == "" {
		mounter.UID = secretMap["gid"]
	} else if secretMap["uid"] != "" {
		mounter.UID = secretMap["uid"]
	}

	updatedOptions := updateMountOptions(mountOptions, secretMap)
	mounter.MountOptions = updatedOptions

	mounter.MounterUtils = mounterUtils

	return mounter
}

func updateMountOptions(dafaultMountOptions []string, secretMap map[string]string) []string {
	mountOptsMap := make(map[string]string)

	// Create map out of array
	for _, e := range dafaultMountOptions {
		opts := strings.Split(e, "=")
		if len(opts) == 2 {
			mountOptsMap[opts[0]] = opts[1]
		}
	}

	stringData, ok := secretMap["mountOptions"]

	if !ok {
		rcloneLogger.Info("No new mountOptions found. Using default mountOptions", zap.Any("default_mount_options", dafaultMountOptions))
		return dafaultMountOptions
	}

	lines := strings.Split(stringData, "\n")

	// Update map
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		opts := strings.Split(line, "=")
		if len(opts) != 2 {
			rcloneLogger.Info("Invalid mount option", zap.String("line", line))
			continue
		}
		mountOptsMap[strings.TrimSpace(opts[0])] = strings.TrimSpace(opts[1])
	}

	// Create array out of map
	updatedOptions := []string{}
	for k, v := range mountOptsMap {
		option := fmt.Sprintf("%s=%s", k, v)
		updatedOptions = append(updatedOptions, option)
	}

	rcloneLogger.Info("Updated rclone Options", zap.Any("updated_options", updatedOptions))

	return updatedOptions
}

func (rclone *RcloneMounter) Mount(ctx context.Context, source string, target string) error {
	reqID := requestid.FromContext(ctx)
	baseLogger, logErr := zap.NewProduction()
	if logErr != nil {
		return fmt.Errorf("failed to create logger: %w", logErr)
	}
	log := logger.WithRequestID(ctx, baseLogger)

	log.Info(fmt.Sprintf("[%s] RcloneMounter Mount started", reqID),
		zap.String("source", source), zap.String("target", target))

	var bucketName string
	var err error

	var configPath string
	if mountWorker {
		configPath = constants.MounterConfigPathOnHost
	} else {
		configPath = constants.MounterConfigPathOnPodRclone
	}

	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	log.Debug(fmt.Sprintf("[%s] Creating rclone config", reqID), zap.String("config_path", configPathWithVolID))

	if err = createConfigWrap(configPathWithVolID, rclone); err != nil {
		log.Error(fmt.Sprintf("[%s] Cannot create rclone config file", reqID), zap.Error(err))
		return err
	}

	if rclone.ObjectPath != "" {
		trimmedPath := strings.TrimPrefix(rclone.ObjectPath, "/")
		bucketName = fmt.Sprintf("%s:%s/%s", remote, rclone.BucketName, trimmedPath)
	} else {
		bucketName = fmt.Sprintf("%s:%s", remote, rclone.BucketName)
	}

	log.Info(fmt.Sprintf("[%s] Formulating mount options", reqID),
		zap.String("bucket_name", bucketName),
		zap.String("auth_type", rclone.AuthType))
	args, wnOp := rclone.formulateMountOptions(bucketName, target, configPathWithVolID)

	if mountWorker {
		log.Info(fmt.Sprintf("[%s] Mount on Worker started", reqID))

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Error marshalling data", reqID), zap.Error(err))
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`, target, bucketName, constants.RClone, jsonData)

		log.Debug(fmt.Sprintf("[%s] Worker mounting payload", reqID), zap.String("payload", payload))

		err = mounterRequest(ctx, payload, "http://unix/api/cos/mount", log)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Failed to mount on worker", reqID), zap.Error(err))
			return err
		}
		log.Info(fmt.Sprintf("[%s] RcloneMounter Mount completed successfully on worker", reqID))
		return nil
	}

	log.Info(fmt.Sprintf("[%s] NodeServer mounting", reqID))
	err = rclone.MounterUtils.FuseMount(target, constants.RClone, args)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] FuseMount failed", reqID), zap.Error(err))
	} else {
		log.Info(fmt.Sprintf("[%s] RcloneMounter Mount completed successfully", reqID))
	}
	return err
}

func (rclone *RcloneMounter) Unmount(ctx context.Context, target string) error {
	reqID := requestid.FromContext(ctx)
	baseLogger, logErr := zap.NewProduction()
	if logErr != nil {
		return fmt.Errorf("failed to create logger: %w", logErr)
	}
	log := logger.WithRequestID(ctx, baseLogger)

	log.Info(fmt.Sprintf("[%s] RcloneMounter Unmount started", reqID), zap.String("target", target))

	if mountWorker {
		log.Info(fmt.Sprintf("[%s] Unmount on Worker started", reqID))

		payload := fmt.Sprintf(`{"path":"%s"}`, target)

		err := mounterRequest(ctx, payload, "http://unix/api/cos/unmount", log)
		if err != nil {
			log.Error(fmt.Sprintf("[%s] Failed to unmount on worker", reqID), zap.Error(err))
			return err
		}

		removeConfigFile(constants.MounterConfigPathOnHost, target)
		log.Info(fmt.Sprintf("[%s] RcloneMounter Unmount completed successfully on worker", reqID))
		return nil
	}

	log.Info(fmt.Sprintf("[%s] NodeServer unmounting", reqID))

	var err error
	err = rclone.MounterUtils.FuseUnmount(target)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] FuseUnmount failed", reqID), zap.Error(err))
		return err
	}

	removeConfigFile(constants.MounterConfigPathOnPodRclone, target)
	log.Info(fmt.Sprintf("[%s] RcloneMounter Unmount completed successfully", reqID))
	return nil
}

func createConfig(configPathWithVolID string, rclone *RcloneMounter) error {
	var accessKey, secretKey, apiKey, envAuth, v2Auth string

	configParams := []string{
		"[" + remote + "]",
		"type = " + s3Type,
		"endpoint = " + rclone.EndPoint,
		"provider = " + cosProvider,
	}

	if rclone.AuthType == "hmac" {
		keys := strings.Split(rclone.AccessKeys, ":")
		accessKey = keys[0]
		secretKey = keys[1]
		envAuth = "true"
		v2Auth = "false"

		configParams = append(configParams, "access_key_id = "+accessKey)
		configParams = append(configParams, "secret_access_key = "+secretKey)

	} else {
		apiKey = rclone.AccessKeys
		v2Auth = "true"
		envAuth = "false"

		configParams = append(configParams, "ibm_api_key = "+apiKey)
		configParams = append(configParams, "ibm_resource_instance_id = "+rclone.serviceInstanceID)
	}

	configParams = append(configParams, "env_auth = "+envAuth)
	configParams = append(configParams, "v2_auth = "+v2Auth)

	if rclone.IAMEndpoint != "" {
		configParams = append(configParams, "ibm_iam_endpoint = "+rclone.IAMEndpoint)
	}

	if rclone.LocConstraint != "" {
		configParams = append(configParams, "location_constraint = "+rclone.LocConstraint)
	}

	configParams = append(configParams, rclone.MountOptions...)

	if err := MakeDir(configPathWithVolID, 0755); // #nosec G301: used for rclone
	err != nil {
		rcloneLogger.Error("RcloneMounter Mount: Cannot create directory", zap.String("path", configPathWithVolID), zap.Error(err))
		return err
	}

	configFile := path.Join(configPathWithVolID, configFileName)
	file, err := CreateFile(configFile) // #nosec G304 used for rclone
	if err != nil {
		rcloneLogger.Error("RcloneMounter Mount: Cannot create file", zap.String("file", configFileName), zap.Error(err))
		return err
	}
	defer func() {
		if err = file.Close(); err != nil {
			rcloneLogger.Error("RcloneMounter Mount: Cannot close file", zap.String("file", configFileName), zap.Error(err))
		}
	}()

	err = Chmod(configFile, 0644) // #nosec G302: used for rclone
	if err != nil {
		rcloneLogger.Error("RcloneMounter Mount: Cannot change permissions on file", zap.String("file", configFileName), zap.Error(err))
		return err
	}

	rcloneLogger.Info("Rclone writing to config")
	datawriter := bufio.NewWriter(file)
	for _, line := range configParams {
		_, err = datawriter.WriteString(line + "\n")
		if err != nil {
			rcloneLogger.Error("RcloneMounter Mount: Could not write to config file", zap.Error(err))
			return err
		}
	}
	err = datawriter.Flush()
	if err != nil {
		return err
	}
	rcloneLogger.Info("Rclone created rclone config file")
	return nil
}

func (rclone *RcloneMounter) formulateMountOptions(bucket, target, configPathWithVolID string) (nodeServerOp []string, workerNodeOp map[string]string) {
	nodeServerOp = []string{
		"mount",
		bucket,
		target,
		"--allow-other",
		"--daemon",
		"--config=" + configPathWithVolID + "/" + configFileName,
		"--log-file=/var/log/rclone.log",
		"--vfs-cache-mode=writes",
	}

	workerNodeOp = map[string]string{
		"allow-other":    "true",
		"daemon":         "true",
		"config":         configPathWithVolID + "/" + configFileName,
		"log-file":       "/var/log/rclone.log",
		"vfs-cache-mode": "writes",
	}

	if rclone.GID != "" {
		gidOpt := "--gid=" + rclone.GID
		nodeServerOp = append(nodeServerOp, gidOpt)

		workerNodeOp["gid"] = rclone.GID
	}
	if rclone.UID != "" {
		uidOpt := "--uid=" + rclone.UID
		nodeServerOp = append(nodeServerOp, uidOpt)

		workerNodeOp["uid"] = rclone.UID
	}
	return
}

func removeRcloneConfigFile(configPath, target string) {
	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	for retry := 1; retry <= maxRetries; retry++ {
		_, err := Stat(configPathWithVolID)
		if err != nil {
			if os.IsNotExist(err) {
				rcloneLogger.Info("removeRcloneConfigFile: Config file directory does not exist", zap.String("path", configPathWithVolID))
				return
			}
			rcloneLogger.Error("removeRcloneConfigFile: Failed to stat path", zap.Int("attempt", retry), zap.String("path", configPathWithVolID), zap.Error(err))
			time.Sleep(constants.Interval)
			continue
		}
		err = RemoveAll(configPathWithVolID)
		if err != nil {
			rcloneLogger.Error("removeRcloneConfigFile: Failed to remove config file path", zap.Int("attempt", retry), zap.String("path", configPathWithVolID), zap.Error(err))
			time.Sleep(constants.Interval)
			continue
		}
		rcloneLogger.Info("removeRcloneConfigFile: Successfully removed config file path", zap.String("path", configPathWithVolID))
		return
	}
	rcloneLogger.Error("removeRcloneConfigFile: Failed to remove config file path after max attempts", zap.Int("max_attempts", maxRetries))
}
