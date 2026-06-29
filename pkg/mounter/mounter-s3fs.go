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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	pkgutils "github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type S3fsMounter struct {
	BucketName     string //From Secret in SC
	ObjectPath     string //From Secret in SC
	EndPoint       string //From Secret in SC
	LocConstraint  string //From Secret in SC
	AuthType       string
	AccessKeys     string
	IAMEndpoint    string
	KpRootKeyCrn   string
	MountOptions   []string
	AddMountParam  string
	MounterUtils   utils.MounterUtils
}

const (
	passFile   = ".passwd-s3fs" // #nosec G101: not password
	maxRetries = 3
)

var (
	writePassWrap = writePass
	removeFile    = removeS3FSCredFile
)

func NewS3fsMounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils, knownS3FSOptions *pkgutils.Set, defaultParams map[string]string) Mounter {
	klog.Info("-newS3fsMounter-")

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

	klog.Infof("newS3fsMounter args:\n\tbucketName: [%s]\n\tobjectPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]\n\tkpRootKeyCrn: [%s]",
		mounter.BucketName, mounter.ObjectPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType, mounter.KpRootKeyCrn)

	updatedOptions, addMountParam := updateS3FSMountOptions(mountOptions, secretMap, knownS3FSOptions, defaultParams)
	mounter.MountOptions = updatedOptions
	mounter.AddMountParam = addMountParam

	mounter.MounterUtils = mounterUtils

	return mounter
}

func (s3fs *S3fsMounter) Mount(source string, target string) error {
	klog.Info("-S3FSMounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

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
		klog.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
		return fmt.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
	}

	if !pathExist {
		if err = MakeDir(metaPath, 0755); // #nosec G301: used for s3fs
		err != nil {
			klog.Errorf("S3FSMounter Mount: Cannot create directory %s: %v", metaPath, err)
			return fmt.Errorf("S3FSMounter Mount: Cannot create directory %s: %v", metaPath, err)
		}
	}

	passwdFile := path.Join(metaPath, passFile)
	if err = writePassWrap(passwdFile, s3fs.AccessKeys); err != nil {
		klog.Errorf("S3FSMounter Mount: Cannot create file %s: %v", passwdFile, err)
		return fmt.Errorf("S3FSMounter Mount: Cannot create file %s: %v", passwdFile, err)
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

	args, wnOp := s3fs.formulateMountOptions(bucketName, target, passwdFile)

	if mountWorker {
		klog.Info(" Mount on Worker started...")

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			klog.Fatalf("Error marshalling data: %v", err)
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`, target, bucketName, constants.S3FS, jsonData)

		klog.Info("Worker Mounting Payload...", payload)

		err = mounterRequest(payload, "http://unix/api/cos/mount")
		if err != nil {
			klog.Error("failed to mount on  worker...", err)
			return err
		}
		return nil
	}
	klog.Info("NodeServer Mounting...")
	return s3fs.MounterUtils.FuseMount(target, constants.S3FS, args)
}

func (s3fs *S3fsMounter) Unmount(target string) error {
	klog.Info("-S3FSMounter Unmount-")
	klog.Infof("Unmount args:\n\ttarget: <%s>", target)

	if mountWorker {
		klog.Info("Unmount on Worker started...")

		payload := fmt.Sprintf(`{"path":"%s"}`, target)

		err := mounterRequest(payload, "http://unix/api/cos/unmount")
		if err != nil {
			klog.Error("failed to unmount on  worker...", err)
			return err
		}

		removeFile(constants.MounterConfigPathOnHost, target)
		return nil
	}
	klog.Info("NodeServer Unmounting...")

	err := s3fs.MounterUtils.FuseUnmount(target)
	if err != nil {
		return err
	}

	removeFile(constants.MounterConfigPathOnPodS3fs, target)
	return nil
}

// GetKnownS3FSOptions returns a Set of known s3fs mount option names used to
// classify options as known (standard s3fs) or unknown (custom for addMountParam)
func GetKnownS3FSOptions() *pkgutils.Set {
	return pkgutils.NewSetWithValues(
		"allow_other", "auto_cache", "cipher_suites",
		"connect_timeout", "curldbg", "dbglevel",
		"default_acl", "disable_noobj_cache", "endpoint",
		"gid", "ibm_iam_auth", "ibm_iam_endpoint",
		"instance_name", "kernel_cache", "max_background",
		"max_dirty_data", "max_stat_cache_size", "mp_umask",
		"multipart_size", "multireq_max", "parallel_count",
		"passwd_file", "ro", "readwrite_timeout",
		"retries", "sigv2", "sigv4",
		"stat_cache_expire", "uid", "umask",
		"url", "use_path_request_style", "use_xattr",
		"tmpdir", "use_cache",
	)
}

// classifyMountOptions separates mount options into known and unknown categories by checking against knownOptions Set
func classifyMountOptions(options []string, knownOptions *pkgutils.Set, knownMap, unknownMap map[string]string) {
	for _, opt := range options {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}

		opts := strings.SplitN(opt, "=", 2)
		optName := strings.TrimSpace(opts[0])

		if optName == "" {
			if opt != "" {
				klog.Infof("Invalid mount option: %s", opt)
			}
			continue
		}

		var optValue string
		if len(opts) == 2 {
			optValue = strings.TrimSpace(opts[1])
		} else {
			optValue = optName
		}

		if knownOptions.Contains(optName) {
			knownMap[optName] = optValue
		} else {
			unknownMap[optName] = optValue
		}
	}
}

func applySecretOverrides(secretMap, mountOptsMap map[string]string) {
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
}

func buildMountOptionsSlice(mountOptsMap, secretMap, defaultParams map[string]string) []string {
	updatedOptions := make([]string, 0, len(mountOptsMap)+len(defaultParams))

	for key, val := range mountOptsMap {
		if newVal, check := secretMap[key]; check {
			if key != val {
				val = fmt.Sprintf("%s=%s", key, newVal)
			} else {
				val = newVal
			}
			updatedOptions = append(updatedOptions, val)
		} else if key != val {
			updatedOptions = append(updatedOptions, fmt.Sprintf("%s=%s", key, val))
		} else {
			updatedOptions = append(updatedOptions, val)
		}
	}

	for key, value := range defaultParams {
		if value != "" {
			if _, exists := mountOptsMap[key]; !exists {
				updatedOptions = append(updatedOptions, fmt.Sprintf("%s=%s", key, value))
			}
		}
	}

	return updatedOptions
}

func buildAddMountParam(unknownOptionsMap map[string]string) string {
	if len(unknownOptionsMap) == 0 {
		return ""
	}

	unknownOptionsList := make([]string, 0, len(unknownOptionsMap))
	for optName, optValue := range unknownOptionsMap {
		if optName == optValue {
			unknownOptionsList = append(unknownOptionsList, optName)
		} else {
			unknownOptionsList = append(unknownOptionsList, fmt.Sprintf("%s=%s", optName, optValue))
		}
	}

	return strings.Join(unknownOptionsList, ",")
}

func updateS3FSMountOptions(defaultMountOp []string, secretMap map[string]string, knownS3FSOptions *pkgutils.Set, defaultParams map[string]string) ([]string, string) {
	mountOptsMap := make(map[string]string)
	unknownOptionsMap := make(map[string]string)

	// Classify StorageClass's mount options into known (standard s3fs) and unknown (custom) categories
	classifyMountOptions(defaultMountOp, knownS3FSOptions, mountOptsMap, unknownOptionsMap)
	applySecretOverrides(secretMap, mountOptsMap)

	if stringData, ok := secretMap["mountOptions"]; ok {
		lines := strings.Split(stringData, "\n")
		// Classify secret's mount options into known (standard s3fs) and unknown (custom) categories
		classifyMountOptions(lines, knownS3FSOptions, mountOptsMap, unknownOptionsMap)
	} else {
		klog.Infof("No new mountOptions found. Using default mountOptions: %v", mountOptsMap)
	}

	updatedOptions := buildMountOptionsSlice(mountOptsMap, secretMap, defaultParams)
	addMountParam := buildAddMountParam(unknownOptionsMap)

	klog.Infof("updated S3fsMounter Options: %v", updatedOptions)
	if addMountParam != "" {
		klog.Infof("addMountParam (unknown options): %s", addMountParam)
	}
	return updatedOptions, addMountParam
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

	// Add unknown mount options to workerNodeOp for mounter service
	if s3fs.AddMountParam != "" {
		workerNodeOp["add-mount-param"] = s3fs.AddMountParam
		klog.Infof("Adding unknown mount options to mounter request: %s", s3fs.AddMountParam)
	}

	return
}

func removeS3FSCredFile(credDir, target string) {
	metaPath := path.Join(credDir, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	for retry := 1; retry <= maxRetries; retry++ {
		_, err := Stat(metaPath)
		if err != nil {
			if os.IsNotExist(err) {
				klog.Infof("removeS3FSCredFile: Password file directory does not exist: %s", metaPath)
				return
			}
			klog.Errorf("removeS3FSCredFile: Attempt %d - Failed to stat path %s: %v", retry, metaPath, err)
			time.Sleep(constants.Interval)
			continue
		}
		err = RemoveAll(metaPath)
		if err != nil {
			klog.Errorf("removeS3FSCredFile: Attempt %d - Failed to remove password file path %s: %v", retry, metaPath, err)
			time.Sleep(constants.Interval)
			continue
		}
		klog.Infof("removeS3FSCredFile: Successfully removed password file path: %s", metaPath)
		return
	}
	klog.Errorf("removeS3FSCredFile: Failed to remove password file after %d attempts", maxRetries)
}
