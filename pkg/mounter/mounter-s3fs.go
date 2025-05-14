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
	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type S3fsMounter struct {
	BucketName    string //From Secret in SC
	ObjPath       string //From Secret in SC
	EndPoint      string //From Secret in SC
	LocConstraint string //From Secret in SC
	AuthType      string
	AccessKeys    string
	KpRootKeyCrn  string
	MountOptions  []string
	MounterUtils  utils.MounterUtils
}

const passFile = ".passwd-s3fs" // #nosec G101: not password

func NewS3fsMounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
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
	if val, check = secretMap["objPath"]; check {
		mounter.ObjPath = val
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

	if apiKey != "" {
		mounter.AccessKeys = fmt.Sprintf(":%s", apiKey)
		mounter.AuthType = "iam"
	} else {
		mounter.AccessKeys = fmt.Sprintf("%s:%s", accessKey, secretKey)
		mounter.AuthType = "hmac"
	}

	klog.Infof("newS3fsMounter args:\n\tbucketName: [%s]\n\tobjPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]\n\tkpRootKeyCrn: [%s]",
		mounter.BucketName, mounter.ObjPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType, mounter.KpRootKeyCrn)

	updatedOptions := updateS3FSMountOptions(mountOptions, secretMap)
	mounter.MountOptions = updatedOptions

	mounter.MounterUtils = mounterUtils

	return mounter
}

func (s3fs *S3fsMounter) Mount(source string, target string) error {
	klog.Info("-S3FSMounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

	var metaRoot string
	if mountWorker {
		metaRoot = constants.MounterConfigPathOnHost
	} else {
		metaRoot = constants.MounterConfigPathOnPod
	}

	var bucketName string
	var pathExist bool
	var err error

	metaPath := path.Join(metaRoot, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if pathExist, err = checkPath(metaPath); err != nil {
		klog.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
		return fmt.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
	}

	if !pathExist {
		if err = mkdirAll(metaPath, 0755); // #nosec G301: used for s3fs
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

	if s3fs.ObjPath != "" {
		bucketName = fmt.Sprintf("%s:/%s", s3fs.BucketName, s3fs.ObjPath)
	} else {
		bucketName = s3fs.BucketName
	}

	args, wnOp := s3fs.formulateMountOptions(bucketName, target, passwdFile)

	if mountWorker {
		klog.Info("Worker Mounting...")

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			klog.Fatalf("Error marshalling data: %v", err)
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`, target, bucketName, constants.S3FS, jsonData)

		klog.Info("Worker Mounting Payload...", payload)

		errResponse, err := createCOSCSIMounterRequest(payload, "http://unix/api/cos/mount")
		klog.Info("Worker Mounting...", errResponse)
		if err != nil {
			return err
		}
		return nil
	}
	klog.Info("NodeServer Mounting...")
	return s3fs.MounterUtils.FuseMount(target, constants.S3FS, args)
}

var writePassFunc = writePass

// Function that wraps writePass
var writePassWrap = func(pwFileName string, pwFileContent string) error {
	return writePassFunc(pwFileName, pwFileContent)
}

func (s3fs *S3fsMounter) Unmount(target string) error {
	klog.Info("-S3FSMounter Unmount-")
	klog.Infof("Unmount args:\n\ttarget: <%s>", target)

	if mountWorker {
		klog.Info("Worker Unmounting...")

		payload := fmt.Sprintf(`{"path":"%s"}`, target)

		errResponse, err := createCOSCSIMounterRequest(payload, "http://unix/api/cos/unmount")
		klog.Info("Worker Unmounting...", errResponse)
		if err != nil {
			return err
		}

		cleanupOfs3fsPasswordFile(target)
		return nil
	}
	klog.Info("NodeServer Unmounting...")
	err := s3fs.MounterUtils.FuseUnmount(target)
	if err != nil {
		return err
	}

	cleanupOfs3fsPasswordFile(target)
	return nil
}

func updateS3FSMountOptions(defaultMountOp []string, secretMap map[string]string) []string {
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
		klog.Infof("No new mountOptions found. Using default mountOptions: %v", mountOptsMap)
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
				klog.Infof("Invalid mount option: %s\n", line)
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
		//klog.Infof("newS3fsMounter mountOption: [%s]", option)
	}

	klog.Infof("updated S3fsMounter Options: %v", updatedOptions)
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
		nodeServerOp = append(nodeServerOp, "-o", "ibm_iam_endpoint="+constants.DefaultIAMEndPoint)

		workerNodeOp["ibm_iam_auth"] = "true"
		workerNodeOp["ibm_iam_endpoint"] = constants.DefaultIAMEndPoint
	} else {
		nodeServerOp = append(nodeServerOp, "-o", "default_acl=private")
		workerNodeOp["default_acl"] = "private"
	}
	return
}

func cleanupOfs3fsPasswordFile(target string) {
	var metaRoot string
	if mountWorker {
		metaRoot = constants.MounterConfigPathOnHost
	} else {
		metaRoot = constants.MounterConfigPathOnPod
	}

	metaPath := path.Join(metaRoot, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	for retry := 1; retry <= 3; retry++ {
		_, err := os.Stat(metaPath)
		if err == nil {
			passwdFile := path.Join(metaPath, passFile)
			err = os.Remove(passwdFile)
			if err != nil {
				klog.Errorf("S3FSMounter Unmount: Cannot remove password file %s: %v", metaPath, err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return
		} else {
			if os.IsNotExist(err) {
				klog.Infof("S3FSMounter Unmount: Password file does not exists%s", metaPath)
				return
			}
			klog.Errorf("S3FSMounter Unmount: Error occurred while fetching path stats for password file")
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}
	return
}
