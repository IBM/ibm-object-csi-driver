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

	// "github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// mntS3Mounter Implements Mounter
type mntS3Mounter struct {
	bucketName    string //From Secret in SC
	objPath       string //From Secret in SC
	endPoint      string //From Secret in SC
	locConstraint string //From Secret in SC
	authType      string
	accessKey     string
	secretKey     string
	mountOptions  []string
	MounterUtils  utils.MounterUtils
}

func NewMntS3Mounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
	klog.Info("-newMntS3Mounter-")

	var (
		val     string
		check   bool
		mounter *mntS3Mounter
	)

	mounter = &mntS3Mounter{}

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
		mounter.accessKey = val
	}
	if val, check = secretMap["secretKey"]; check {
		mounter.secretKey = val
	}

	klog.Infof("newMntS3Mounter args:\n\tbucketName: [%s]\n\tobjPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.bucketName, mounter.objPath, mounter.endPoint, mounter.locConstraint, mounter.authType)

	mounter.mountOptions = mountOptions
	mounter.MounterUtils = mounterUtils

	return mounter
}

const (
	mntS3Cmd      = "mount-s3"
	metaRootMntS3 = "/var/lib/ibmc-mntS3"
)

func (mntS3 *mntS3Mounter) Stage(stagePath string) error {
	return nil
}
func (mntS3 *mntS3Mounter) Unstage(stagePath string) error {
	return nil
}
func (mntS3 *mntS3Mounter) Mount(source string, target string) error {
	klog.Info("-MntS3Mounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)
	var pathExist bool
	var err error
	metaPath := path.Join(metaRootMntS3, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if pathExist, err = checkPath(metaPath); err != nil {
		klog.Errorf("MntS3Mounter Mount: Cannot stat directory %s: %v", metaPath, err)
		return err
	}

	if !pathExist {
		if err = os.MkdirAll(metaPath, 0755); // #nosec G301: used for mntS3
		err != nil {
			klog.Errorf("MntS3Mounter Mount: Cannot create directory %s: %v", metaPath, err)
			return err
		}
	}

	os.Setenv("AWS_ACCESS_KEY_ID", mntS3.accessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", mntS3.secretKey)

	args := []string{
		fmt.Sprintf("--endpoint-url=%v", mntS3.endPoint),
		mntS3.bucketName,
		target,
	}

	if mntS3.objPath != "" {
		args = append(args, fmt.Sprintf("--prefix %s", mntS3.objPath))
	}
	return mntS3.MounterUtils.FuseMount(target, mntS3Cmd, args)
}
func (mntS3 *mntS3Mounter) Unmount(target string) error {
	klog.Info("-MntS3Mounter Unmount-")
	metaPath := path.Join(metaRootMntS3, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	err := os.RemoveAll(metaPath)
	if err != nil {
		return err
	}

	return mntS3.MounterUtils.FuseUnmount(target)
}
