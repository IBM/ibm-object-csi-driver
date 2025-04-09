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
// MountpointMounter Implements Mounter
type MountpointMounter struct {
	BucketName   string //From Secret in SC
	ObjPath      string //From Secret in SC
	EndPoint     string //From Secret in SC
	AccessKey    string
	SecretKey    string
	MountOptions []string
	MounterUtils utils.MounterUtils
}

func NewMountpointMounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
	klog.Info("-newMountpointMounter-")

	var (
		val     string
		check   bool
		mounter *MountpointMounter
	)

	mounter = &MountpointMounter{}

	if val, check = secretMap["cosEndpoint"]; check {
		mounter.EndPoint = val
	}
	if val, check = secretMap["bucketName"]; check {
		mounter.BucketName = val
	}
	if val, check = secretMap["objPath"]; check {
		mounter.ObjPath = val
	}
	if val, check = secretMap["accessKey"]; check {
		mounter.AccessKey = val
	}
	if val, check = secretMap["secretKey"]; check {
		mounter.SecretKey = val
	}

	klog.Infof("newMntS3Mounter args:\n\tbucketName: [%s]\n\tobjPath: [%s]\n\tendPoint: [%s]",
		mounter.BucketName, mounter.ObjPath, mounter.EndPoint)

	mounter.MountOptions = mountOptions
	mounter.MounterUtils = mounterUtils

	return mounter
}

const (
	mntS3Cmd      = "mount-s3"
	metaRootMntS3 = "/var/lib/ibmc-mntS3"
)

func (mntS3 *MountpointMounter) Stage(stagePath string) error {
	return nil
}
func (mntS3 *MountpointMounter) Unstage(stagePath string) error {
	return nil
}
func (mntS3 *MountpointMounter) Mount(source string, target string) error {
	klog.Info("-MountpointMounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)
	var pathExist bool
	var err error
	metaPath := path.Join(metaRootMntS3, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if pathExist, err = checkPath(metaPath); err != nil {
		klog.Errorf("MountpointMounter Mount: Cannot stat directory %s: %v", metaPath, err)
		return err
	}

	if !pathExist {
		if err = mkdirAll(metaPath, 0755); // #nosec G301: used for mntS3
		err != nil {
			klog.Errorf("MountpointMounter Mount: Cannot create directory %s: %v", metaPath, err)
			return err
		}
	}

	os.Setenv("AWS_ACCESS_KEY_ID", mntS3.AccessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", mntS3.SecretKey)

	args := []string{
		fmt.Sprintf("--endpoint-url=%v", mntS3.EndPoint),
		mntS3.BucketName,
		target,
	}

	if mntS3.ObjPath != "" {
		args = append(args, fmt.Sprintf("--prefix %s", mntS3.ObjPath))
	}
	return mntS3.MounterUtils.FuseMount(target, mntS3Cmd, args)
}
func (mntS3 *MountpointMounter) Unmount(target string) error {
	klog.Info("-MountpointMounter Unmount-")
	metaPath := path.Join(metaRootMntS3, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	err := os.RemoveAll(metaPath)
	if err != nil {
		return err
	}

	return mntS3.MounterUtils.FuseUnmount(target)
}
