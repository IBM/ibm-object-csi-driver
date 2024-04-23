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
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type s3fsMounter struct {
	bucketName    string //From Secret in SC
	objPath       string //From Secret in SC
	endPoint      string //From Secret in SC
	locConstraint string //From Secret in SC
	authType      string
	accessKeys    string
	kpRootKeyCrn  string
	mountOptions  []string
}

const (
	s3fsCmd  = "s3fs"
	metaRoot = "/var/lib/ibmc-s3fs"
	passFile = ".passwd-s3fs" // #nosec G101: not password
	// defaultIAMEndPoint is the default URL of the IBM IAM endpoint
	defaultIAMEndPoint = "https://iam.cloud.ibm.com"
)

func newS3fsMounter(secretMap map[string]string, mountOptions []string) (Mounter, error) {
	klog.Info("-newS3fsMounter-")

	var (
		val       string
		check     bool
		accessKey string
		secretKey string
		apiKey    string
		mounter   *s3fsMounter
	)

	mounter = &s3fsMounter{}

	if val, check = secretMap["cosEndpoint"]; check {
		mounter.endPoint = val
	}
	if val, check = secretMap["locationConstraint"]; check {
		mounter.locConstraint = val
	}
	if val, check = secretMap["bucketName"]; check {
		mounter.bucketName = val
	}
	if val, check = secretMap["objPath"]; check {
		mounter.objPath = val
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
		mounter.kpRootKeyCrn = val
	}

	if apiKey != "" {
		mounter.accessKeys = fmt.Sprintf(":%s", apiKey)
		mounter.authType = "iam"
	} else {
		mounter.accessKeys = fmt.Sprintf("%s:%s", accessKey, secretKey)
		mounter.authType = "hmac"
	}

	klog.Infof("newS3fsMounter args:\n\tbucketName: [%s]\n\tobjPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]kpRootKeyCrn: [%s]",
		mounter.bucketName, mounter.objPath, mounter.endPoint, mounter.locConstraint, mounter.authType, mounter.kpRootKeyCrn)

	updatedOptions, err := updateS3FSMountOptions(mountOptions, secretMap)
	if err != nil {
		klog.Infof("Problems with retrieving secret map dynamically %v", err)
	}
	mounter.mountOptions = updatedOptions

	return mounter, nil
}

func (s3fs *s3fsMounter) Stage(stageTarget string) error {
	klog.Info("-S3FSMounter Stage-")
	return nil
}

func (s3fs *s3fsMounter) Unstage(stageTarget string) error {
	klog.Info("-S3FSMounter Unstage-")
	return nil
}

func (s3fs *s3fsMounter) Mount(source string, target string) error {
	klog.Info("-S3FSMounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)
	var bucketName string
	var pathExist bool
	var err error
	metaPath := path.Join(metaRoot, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if pathExist, err = checkPath(metaPath); err != nil {
		klog.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
		return fmt.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
	}

	if !pathExist {
		if err = os.MkdirAll(metaPath, 0755); // #nosec G301: used for s3fs
		err != nil {
			klog.Errorf("S3FSMounter Mount: Cannot create directory %s: %v", metaPath, err)
			return fmt.Errorf("S3FSMounter Mount: Cannot create directory %s: %v", metaPath, err)
		}
	}

	passwdFile := path.Join(metaPath, passFile)
	if err = writePass(passwdFile, s3fs.accessKeys); err != nil {
		klog.Errorf("S3FSMounter Mount: Cannot create file %s: %v", passwdFile, err)
		return fmt.Errorf("S3FSMounter Mount: Cannot create file %s: %v", passwdFile, err)
	}

	if s3fs.objPath != "" {
		bucketName = fmt.Sprintf("%s:/%s", s3fs.bucketName, s3fs.objPath)
	} else {
		bucketName = s3fs.bucketName
	}

	args := []string{
		bucketName,
		target,
		"-o", "sigv2",
		"-o", "use_path_request_style",
		"-o", fmt.Sprintf("passwd_file=%s", passwdFile),
		"-o", fmt.Sprintf("url=%s", s3fs.endPoint),
		"-o", fmt.Sprintf("endpoint=%s", s3fs.locConstraint),
		"-o", "allow_other",
		"-o", "mp_umask=002",
	}

	for _, val := range s3fs.mountOptions {
		args = append(args, "-o")
		args = append(args, val)
	}

	if s3fs.authType != "hmac" {
		args = append(args, "-o", "ibm_iam_auth")
		args = append(args, "-o", "ibm_iam_endpoint="+defaultIAMEndPoint)
	} else {
		args = append(args, "-o", "default_acl=private")
	}
	return fuseMount(target, s3fsCmd, args)
}

func (s3fs *s3fsMounter) Unmount(target string) error {
	klog.Info("-S3FSMounter Unmount-")
	metaPath := path.Join(metaRoot, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	err := os.RemoveAll(metaPath)
	if err != nil {
		return err
	}
	statsUtil := &(utils.VolumeStatsUtils{})
	return statsUtil.FuseUnmount(target)
}

func updateS3FSMountOptions(defaultMountOp []string, secretMap map[string]string) ([]string, error) {
	mountOptsMap := make(map[string]string)

	// Create map out of array
	for _, val := range defaultMountOp {
		if strings.TrimSpace(val) == "" {
			continue
		}
		opts := strings.Split(val, "=")
		if len(opts) == 2 {
			mountOptsMap[opts[0]] = opts[1]
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
			if len(opts) != 2 {
				klog.Infof("Invalid mount option: %s\n", line)
				continue
			}
			mountOptsMap[strings.TrimSpace(opts[0])] = strings.TrimSpace(opts[1])
		}
	}

	// Create array out of map
	updatedOptions := []string{}
	for k, v := range mountOptsMap {
		option := fmt.Sprintf("%s=%s", k, v)
		if k == "cache" {
			option = v
		}
		updatedOptions = append(updatedOptions, option)
		klog.Infof("newS3fsMounter mountOption: [%s]", option)
	}

	klog.Infof("S3fsMounter Options: %v", updatedOptions)
	return updatedOptions, nil
}
