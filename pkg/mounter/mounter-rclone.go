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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// rcloneMounter Implements Mounter
type RcloneMounter struct {
	BucketName    string //From Secret in SC
	ObjPath       string //From Secret in SC
	EndPoint      string //From Secret in SC
	LocConstraint string //From Secret in SC
	AuthType      string
	AccessKeys    string
	KpRootKeyCrn  string
	UID           string
	GID           string
	MountOptions  []string
	MounterUtils  utils.MounterUtils
}

const (
	configFileName = "rclone.conf"
	remote         = "ibmcos"
	s3Type         = "s3"
	cosProvider    = "IBMCOS"
	envAuth        = "true"
)

var createConfigWrap = createConfig

func NewRcloneMounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
	klog.Info("-newRcloneMounter-")

	var (
		val       string
		check     bool
		accessKey string
		secretKey string
		// apiKey    string
		mounter *RcloneMounter
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
	if val, check = secretMap["objPath"]; check {
		mounter.ObjPath = val
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

	// Since IAM support for rClone is not there and api key is required param now, commented below piece of code
	// Uncommnet when IAM support for rClone is available

	// if val, check = secretMap["apiKey"]; check {
	// 	apiKey = val
	// }

	// if apiKey != "" {
	// 	mounter.AccessKeys = fmt.Sprintf(":%s", apiKey)
	// 	mounter.AuthType = "iam"
	// } else {
	mounter.AccessKeys = fmt.Sprintf("%s:%s", accessKey, secretKey)
	mounter.AuthType = "hmac"
	// }

	if val, check = secretMap["gid"]; check {
		mounter.GID = val
	}
	if secretMap["gid"] != "" && secretMap["uid"] == "" {
		mounter.UID = secretMap["gid"]
	} else if secretMap["uid"] != "" {
		mounter.UID = secretMap["uid"]
	}

	klog.Infof("newRcloneMounter args:\n\tbucketName: [%s]\n\tobjPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.BucketName, mounter.ObjPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType)

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
		klog.Infof("No new mountOptions found. Using default mountOptions: %v", dafaultMountOptions)
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
			klog.Infof("Invalid mount option: %s\n", line)
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

	klog.Infof("Updated rclone Options: %v", updatedOptions)

	return updatedOptions
}

func (rclone *RcloneMounter) Mount(source string, target string) error {
	klog.Info("-RcloneMounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

	var bucketName string
	var err error

	var configPath string
	if mountWorker {
		configPath = constants.MounterConfigPathOnHost
	} else {
		configPath = constants.MounterConfigPathOnPodRclone
	}

	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	if err = createConfigWrap(configPathWithVolID, rclone); err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot create rclone config file %v", err)
		return err
	}

	if rclone.ObjPath != "" {
		bucketName = fmt.Sprintf("%s:%s/%s", remote, rclone.BucketName, rclone.ObjPath)
	} else {
		bucketName = fmt.Sprintf("%s:%s", remote, rclone.BucketName)
	}

	args, wnOp := rclone.formulateMountOptions(bucketName, target, configPathWithVolID)

	if mountWorker {
		klog.Info("Mount on Worker started...")

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			klog.Fatalf("Error marshalling data: %v", err)
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`, target, bucketName, constants.RClone, jsonData)

		response, err := mounterRequest(payload, "http://unix/api/cos/mount")
		klog.Info("Worker Mounting...", response)
		if err != nil {
			return err
		}
		return nil
	}
	klog.Info("NodeServer Mounting...")
	return rclone.MounterUtils.FuseMount(target, constants.RClone, args)
}

func (rclone *RcloneMounter) Unmount(target string) error {
	klog.Info("-RcloneMounter Unmount-")

	if mountWorker {
		klog.Info("Unmount on Worker started...")

		payload := fmt.Sprintf(`{"path":"%s"}`, target)

		response, err := mounterRequest(payload, "http://unix/api/cos/unmount")
		klog.Info("Worker Unmounting...", response)
		if err != nil {
			return err
		}

		removeRcloneConfigFile(constants.MounterConfigPathOnHost, target)
		return nil
	}
	klog.Info("NodeServer Unmounting...")

	err := rclone.MounterUtils.FuseUnmount(target)
	if err != nil {
		return err
	}

	removeRcloneConfigFile(constants.MounterConfigPathOnPodRclone, target)
	return nil
}

func createConfig(configPathWithVolID string, rclone *RcloneMounter) error {
	var accessKey, secretKey string
	keys := strings.Split(rclone.AccessKeys, ":")
	if len(keys) == 2 {
		accessKey = keys[0]
		secretKey = keys[1]
	}
	configParams := []string{
		"[" + remote + "]",
		"type = " + s3Type,
		"endpoint = " + rclone.EndPoint,
		"provider = " + cosProvider,
		"env_auth = " + envAuth,
		"access_key_id = " + accessKey,
		"secret_access_key = " + secretKey,
	}

	if rclone.LocConstraint != "" {
		configParams = append(configParams, "location_constraint = "+rclone.LocConstraint)
	}

	configParams = append(configParams, rclone.MountOptions...)

	if err := MakeDir(configPathWithVolID, 0755); // #nosec G301: used for rclone
	err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot create directory %s: %v", configPathWithVolID, err)
		return err
	}

	configFile := path.Join(configPathWithVolID, configFileName)
	file, err := CreateFile(configFile) // #nosec G304 used for rclone
	if err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot create file %s: %v", configFileName, err)
		return err
	}
	defer func() {
		if err = file.Close(); err != nil {
			klog.Errorf("RcloneMounter Mount: Cannot close file %s: %v", configFileName, err)
		}
	}()

	err = Chmod(configFile, 0644) // #nosec G302: used for rclone
	if err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot change permissions on file  %s: %v", configFileName, err)
		return err
	}

	klog.Info("-Rclone writing to config-")
	datawriter := bufio.NewWriter(file)
	for _, line := range configParams {
		_, err = datawriter.WriteString(line + "\n")
		if err != nil {
			klog.Errorf("RcloneMounter Mount: Could not write to config file: %v", err)
			return err
		}
	}
	err = datawriter.Flush()
	if err != nil {
		return err
	}
	klog.Info("-Rclone created rclone config file-")
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
	}

	workerNodeOp = map[string]string{
		"allow-other": "true",
		"daemon":      "true",
		"config":      configPathWithVolID + "/" + configFileName,
		"log-file":    "/var/log/rclone.log",
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
		_, err := os.Stat(configPathWithVolID)
		if err != nil {
			if os.IsNotExist(err) {
				klog.Infof("removeRcloneConfigFile: Password file directory does not exist: %s", configPathWithVolID)
				return
			}
			klog.Errorf("removeRcloneConfigFile: Attempt %d - Failed to stat path %s: %v", retry, configPathWithVolID, err)
			time.Sleep(constants.Interval)
			continue
		}
		configFile := path.Join(configPathWithVolID, configFileName)
		err = os.Remove(configFile)
		if err != nil {
			klog.Errorf("removeRcloneConfigFile: Attempt %d - Failed to remove password file %s: %v", retry, configFile, err)
			time.Sleep(constants.Interval)
			continue
		}
		klog.Infof("removeRcloneConfigFile: Successfully removed config file: %s", configFile)
		return
	}
	klog.Errorf("removeRcloneConfigFile: Failed to remove config file after %d attempts", maxRetries)
}
