/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2026 All Rights Reserved.
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

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

type MountS3Mounter struct {
	BucketName    string
	ObjectPath    string
	EndPoint      string
	LocConstraint string
	AuthType      string
	AccessKey     string
	SecretKey     string
	UID           string
	GID           string
	LogLevel      string
	CacheDir      string
	ReadOnly      bool
	MountOptions  []string
	MounterUtils  utils.MounterUtils
}

const (
	s3MountLogDirectory = "/var/log/mount-s3"

	s3MountCredentialsFile = "credentials"
	s3MountConfigFile      = "config"
	s3MountAwsProfile      = "default"
)

var (
	removeS3ConfigFile = removeConfigDir

	// User-supplied flags that are hardcoded, driver-managed or unsupported, with the reason.
	s3MountIgnoredOptions = map[string]string{
		"allow-other":        "is always enabled and cannot be overridden",
		"umask":              "is not supported by mount-s3, use --dir-mode / --file-mode instead",
		"log-directory":      fmt.Sprintf("is hardcoded to %q and cannot be overridden", s3MountLogDirectory),
		"force-path-style":   "is hardcoded (required for IBM COS) and cannot be overridden",
		"read-only":          "is set via MounterParams and cannot be overridden via mount options",
		"allow-delete":       "is hardcoded (enabled unless read-only) and cannot be overridden",
		"allow-overwrite":    "is hardcoded (enabled unless read-only) and cannot be overridden",
		"incremental-upload": "requires AWS S3 Express One Zone and causes data loss on IBM COS",
	}
)

type MountS3MounterParams struct {
	SecretMap    map[string]string
	MountOptions []string
	MounterUtils utils.MounterUtils
	Gid          string
	ReadOnly     bool
}

// NewMountS3Mounter builds the mounter. Precedence (lowest to highest): secretMap
// keys, StorageClass mountOptions, secret mountOptions.
func NewMountS3Mounter(params MountS3MounterParams) Mounter {
	secretMap := params.SecretMap
	mountOptions := params.MountOptions
	mounterUtils := params.MounterUtils
	gid := params.Gid
	readOnly := params.ReadOnly

	klog.Info("-newMountS3Mounter-")

	mounter := &MountS3Mounter{}
	mounter.MounterUtils = mounterUtils

	if val, ok := secretMap["accessKey"]; ok {
		mounter.AccessKey = val
	}
	if val, ok := secretMap["secretKey"]; ok {
		mounter.SecretKey = val
	}
	if val, ok := secretMap["bucketName"]; ok {
		mounter.BucketName = val
	}
	if val, ok := secretMap["objectPath"]; ok {
		mounter.ObjectPath = val
	}
	if val, ok := secretMap["cosEndpoint"]; ok {
		mounter.EndPoint = val
	}
	if val, ok := secretMap["locationConstraint"]; ok {
		mounter.LocConstraint = val
	}

	if gid != "" && secretMap["uid"] == "" {
		mounter.UID = gid
	} else if val := secretMap["uid"]; val != "" {
		mounter.UID = val
	}
	if val := secretMap["gid"]; val != "" {
		mounter.GID = val
	} else {
		mounter.GID = gid
	}

	mounter.ReadOnly = readOnly
	mounter.AuthType = "hmac"

	mounter.MountOptions = updateMountS3Options(mounter, mountOptions, secretMap)

	klog.Infof("newMountS3Mounter args:\n\tbucketName: [%s]\n\tobjectPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.BucketName, mounter.ObjectPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType)

	return mounter
}

// updateMountS3Options merges StorageClass and secret mountOptions into one map keyed by
// flag name (secret wins on the same key).
// uid, gid, log-level and cache are applied to the mounter, driver-managed flags are
// dropped, and the rest is returned as "--key[=value]" passthrough flags.
func updateMountS3Options(mounter *MountS3Mounter, scOptions []string, secretMap map[string]string) []string {
	mountOptsMap := make(map[string]string)

	addOptions := func(opts []string) {
		for _, opt := range opts {
			opt = strings.TrimRight(strings.TrimSpace(opt), ",")
			if opt == "" || strings.HasPrefix(opt, "#") {
				continue
			}
			key, value, _ := strings.Cut(strings.TrimPrefix(opt, "--"), "=")
			mountOptsMap[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	addOptions(scOptions)
	if stringData, ok := secretMap["mountOptions"]; ok {
		addOptions(strings.Split(stringData, "\n"))
	} else {
		klog.Infof("No new mountOptions found. Using default mountOptions: %v", scOptions)
	}

	updatedOptions := make([]string, 0, len(mountOptsMap))
	for key, value := range mountOptsMap {
		if reason, ignored := s3MountIgnoredOptions[key]; ignored {
			klog.Warningf("updateMountS3Options: '%s' %s. Ignoring.", key, reason)
			continue
		}

		switch {
		case key == "uid" && value != "":
			mounter.UID = value
		case key == "gid" && value != "":
			mounter.GID = value
		case key == "log-level" && value != "":
			mounter.LogLevel = value
		case key == "cache" && value != "":
			mounter.CacheDir = value
		case key == "uid" || key == "gid" || key == "log-level" || key == "cache":
			klog.Warningf("updateMountS3Options: '%s' requires a value. Ignoring.", key)
		case value == "":
			updatedOptions = append(updatedOptions, "--"+key)
		default:
			updatedOptions = append(updatedOptions, "--"+key+"="+value)
		}
	}

	klog.Infof("Updated mount-s3 Options: %v", updatedOptions)
	return updatedOptions
}

func (s3 *MountS3Mounter) Mount(source string, target string) error {
	klog.Info("-MountS3Mounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

	var configPath string
	if mountWorker {
		configPath = constants.MounterConfigPathOnHost
	} else {
		configPath = constants.MounterConfigPathOnPodS3Mount
	}

	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if err := createS3MountConfig(configPathWithVolID, s3); err != nil {
		klog.Errorf("MountS3Mounter Mount: Cannot create config file: %v", err)
		return err
	}

	bucketName := s3.BucketName
	if s3.ObjectPath != "" {
		trimmedPath := strings.TrimPrefix(s3.ObjectPath, "/")
		bucketName = fmt.Sprintf("%s/%s", s3.BucketName, trimmedPath)
	}

	args, envVars, wnOp := s3.formulateMountOptions(bucketName, target, configPathWithVolID)

	if mountWorker {
		klog.Info("Mount on Worker started...")

		mountReq := struct {
			Path    string            `json:"path"`
			Bucket  string            `json:"bucket"`
			Mounter string            `json:"mounter"`
			Args    map[string]string `json:"args"`
		}{
			Path:    target,
			Bucket:  bucketName,
			Mounter: constants.AMAZONS3MOUNTER,
			Args:    wnOp,
		}
		payloadBytes, err := json.Marshal(mountReq)
		if err != nil {
			klog.Errorf("Error marshalling mount request: %v", err)
			return err
		}

		klog.Infof("MountS3Mounter: worker payload: %s", payloadBytes)

		err = mounterRequest(string(payloadBytes), "http://unix/api/cos/mount")
		if err != nil {
			klog.Errorf("failed to mount on worker: %v", err)
			return err
		}
		return nil
	}

	klog.Info("NodeServer Mounting...")

	return s3.MounterUtils.FuseMountWithEnv(target, constants.MountS3BinaryPath, args, envVars)
}

func (s3 *MountS3Mounter) Unmount(target string) error {
	klog.Info("-MountS3Mounter Unmount-")

	if mountWorker {
		klog.Info("Unmount on Worker started...")

		unmountReq := struct {
			Path string `json:"path"`
		}{
			Path: target,
		}
		payloadBytes, err := json.Marshal(unmountReq)
		if err != nil {
			klog.Errorf("Error marshalling unmount request: %v", err)
			return err
		}

		err = mounterRequest(string(payloadBytes), "http://unix/api/cos/unmount")
		if err != nil {
			klog.Errorf("failed to unmount on worker: %v", err)
			return err
		}

		removeS3ConfigFile(constants.MounterConfigPathOnHost, target)
		return nil
	}

	klog.Info("NodeServer Unmounting...")
	err := s3.MounterUtils.FuseUnmount(target)
	if err != nil {
		return err
	}

	removeS3ConfigFile(constants.MounterConfigPathOnPodS3Mount, target)
	return nil
}

func createS3MountConfig(configPathWithVolID string, s3 *MountS3Mounter) error {
	if err := MakeDir(configPathWithVolID, 0755); err != nil { // #nosec G301 -- config directory needs to be readable by mount-s3 process
		klog.Errorf("MountS3Mounter: Cannot create config dir %s: %v", configPathWithVolID, err)
		return err
	}

	credFile := path.Join(configPathWithVolID, s3MountCredentialsFile)
	credParams := []string{
		"[" + s3MountAwsProfile + "]",
		"aws_access_key_id = " + s3.AccessKey,
		"aws_secret_access_key = " + s3.SecretKey,
	}
	if err := writeConfigFile(credFile, credParams); err != nil {
		klog.Errorf("MountS3Mounter: Cannot write credentials file: %v", err)
		return err
	}

	region := s3.LocConstraint
	configParams := []string{
		"[" + s3MountAwsProfile + "]",
		"region = " + region,
	}
	if s3.EndPoint != "" {
		configParams = append(configParams, "endpoint_url = "+s3.EndPoint)
	}
	cfgFile := path.Join(configPathWithVolID, s3MountConfigFile)
	if err := writeConfigFile(cfgFile, configParams); err != nil {
		klog.Errorf("MountS3Mounter: Cannot write config file: %v", err)
		return err
	}

	klog.Infof("MountS3Mounter: Created config files at %s", configPathWithVolID)
	return nil
}

func writeConfigFile(filePath string, lines []string) error {
	f, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600) // #nosec G304 -- file path is constructed from controlled configPathWithVolID and constant filename
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			klog.Errorf("writeConfigFile: cannot close file %s: %v", filePath, cerr)
		}
	}()

	w := bufio.NewWriter(f)
	for _, line := range lines {
		if _, err := w.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("cannot write to file %s: %w", filePath, err)
		}
	}
	return w.Flush()
}

func (s3 *MountS3Mounter) formulateMountOptions(bucket, target, configPathWithVolID string) (nodeServerOp []string, envVars []string, workerNodeOp map[string]string) {
	nodeServerOp = []string{bucket, target, "--allow-other"}

	credFilePath := path.Join(configPathWithVolID, s3MountCredentialsFile)
	cfgFilePath := path.Join(configPathWithVolID, s3MountConfigFile)
	envVars = []string{
		"AWS_SHARED_CREDENTIALS_FILE=" + credFilePath,
		"AWS_CONFIG_FILE=" + cfgFilePath,
	}

	workerNodeOp = map[string]string{
		"allow-other":          "true",
		"aws-credentials-file": credFilePath,
		"aws-config-file":      cfgFilePath,
	}

	if s3.EndPoint != "" {
		nodeServerOp = append(nodeServerOp, "--endpoint-url="+s3.EndPoint)
		workerNodeOp["endpoint-url"] = s3.EndPoint
	}
	if s3.LocConstraint != "" {
		nodeServerOp = append(nodeServerOp, "--region="+s3.LocConstraint)
		workerNodeOp["region"] = s3.LocConstraint
	}
	if s3.UID != "" {
		nodeServerOp = append(nodeServerOp, "--uid="+s3.UID)
		workerNodeOp["uid"] = s3.UID
	}
	if s3.GID != "" {
		nodeServerOp = append(nodeServerOp, "--gid="+s3.GID)
		workerNodeOp["gid"] = s3.GID
	}

	switch s3.LogLevel {
	case "debug":
		nodeServerOp = append(nodeServerOp, "--debug")
		workerNodeOp["log-level"] = "debug"
	case "debug-crt":
		nodeServerOp = append(nodeServerOp, "--debug-crt")
		workerNodeOp["log-level"] = "debug-crt"
	case "no-log":
		nodeServerOp = append(nodeServerOp, "--no-log")
		workerNodeOp["log-level"] = "no-log"
	default:
		if s3.LogLevel != "" {
			klog.Warningf("MountS3Mounter: unsupported log-level %q. "+
				"Supported: 'debug', 'debug-crt', 'no-log'. Ignoring.", s3.LogLevel)
		}
	}

	nodeServerOp = append(nodeServerOp, "--log-directory="+s3MountLogDirectory)
	workerNodeOp["log-directory"] = s3MountLogDirectory

	if s3.CacheDir != "" {
		nodeServerOp = append(nodeServerOp, "--cache="+s3.CacheDir)
		workerNodeOp["cache"] = s3.CacheDir
	}

	nodeServerOp = append(nodeServerOp, "--force-path-style")
	workerNodeOp["force-path-style"] = "true"

	// read-only suppresses allow-delete and allow-overwrite.
	if s3.ReadOnly {
		nodeServerOp = append(nodeServerOp, "--read-only")
		workerNodeOp["read-only"] = "true"
	} else {
		nodeServerOp = append(nodeServerOp, "--allow-delete", "--allow-overwrite")
		workerNodeOp["allow-delete"] = "true"
		workerNodeOp["allow-overwrite"] = "true"
	}

	// Passthrough flags have no typed field on the server, so the worker receives
	// them as a comma-separated "add-mount-param" string.
	nodeServerOp = append(nodeServerOp, s3.MountOptions...)
	if len(s3.MountOptions) > 0 {
		workerNodeOp["add-mount-param"] = strings.Join(s3.MountOptions, ",")
	}

	klog.Infof("MountS3 nodeServerOp: %v", nodeServerOp)
	klog.Infof("MountS3 workerNodeOp: %+v", workerNodeOp)

	return nodeServerOp, envVars, workerNodeOp
}
