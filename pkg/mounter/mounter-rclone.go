package mounter

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"strings"

	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// rcloneMounter Implements Mounter
type rcloneMounter struct {
	bucketName   string //From Secret in SC
	objPath      string //From Secret in SC
	endPoint     string //From Secret in SC
	regnClass    string //From Secret in SC
	accessKey    string
	accessSecret string
	mountOptions []string
}

func newRcloneMounter(secretMap map[string]string, mountOptions []string) (Mounter, error) {
	klog.Info("-newRcloneMounter-")

	bucket := secretMap["bucket-name"]
	objpath := secretMap["obj-path"]
	endpoint := secretMap["cos-endpoint"]
	region := secretMap["regn-class"]
	accessKey := secretMap["access-key"]
	secretKey := secretMap["secret-key"]

	klog.Infof("newRcloneMounter args:bucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>", bucket, objpath, endpoint, region)

	return &rcloneMounter{
		bucketName:   bucket,
		objPath:      objpath,
		endPoint:     endpoint,
		regnClass:    region,
		accessKey:    accessKey,
		accessSecret: secretKey,
		mountOptions: mountOptions,
	}, nil
}

const (
	rcloneCmd      = "rclone"
	metaRootRclone = "/var/lib/ibmc-rclone"
	configPath     = "/root/.config/rclone"
	configFileName = "rclone.conf"
	remote         = "ibmcos"
	s3Type         = "s3"
	cosProvider    = "IBMCOS"
	envAuth        = "true"
)

func (rclone *rcloneMounter) Stage(stagePath string) error {
	return nil
}
func (rclone *rcloneMounter) Unstage(stagePath string) error {
	return nil
}
func (rclone *rcloneMounter) Mount(source string, target string) error {
	klog.Info("-RcloneMounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)
	var bucketName string
	var pathExist bool
	var err error
	metaPath := path.Join(metaRootRclone, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if pathExist, err = checkPath(metaPath); err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot stat directory %s: %v", metaPath, err)
		return err
	}

	if !pathExist {
		if err = os.MkdirAll(metaPath, 0755); err != nil {
			klog.Errorf("RcloneMounter Mount: Cannot create directory %s: %v", metaPath, err)
			return err
		}
	}

	if err = rclone.createConfig(); err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot create rclone config file %v", err)
		return err
	}

	if rclone.objPath != "" {
		bucketName = fmt.Sprintf("%s:%s/%s", remote, rclone.bucketName, rclone.objPath)
	} else {
		bucketName = fmt.Sprintf("%s:%s", remote, rclone.bucketName)
	}

	args := []string{
		"mount",
		bucketName,
		target,
		"--daemon",
	}
	return fuseMount(target, rcloneCmd, args)
}
func (rclone *rcloneMounter) Unmount(target string) error {
	klog.Info("-RcloneMounter Unmount-")
	metaPath := path.Join(metaRootRclone, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	os.RemoveAll(metaPath)

	return FuseUnmount(target)
}

func (rclone *rcloneMounter) createConfig() error {
	configParams := []string{
		"[" + remote + "]",
		"type = " + s3Type,
		"endpoint = " + rclone.endPoint,
		"provider = " + cosProvider,
		"env_auth = " + envAuth,
		"location_constraint = " + rclone.regnClass,
		"access_key_id = " + rclone.accessKey,
		"secret_access_key = " + rclone.accessSecret,
	}

	for _, val := range rclone.mountOptions {
		val = strings.Replace(val, "=", " = ", 1)
		configParams = append(configParams, val)
	}

	if err := os.MkdirAll(configPath, 0755); err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot create directory %s: %v", configPath, err)
		return err
	}

	configFile := path.Join(configPath, configFileName)
	file, err := os.Create(configFile)
	if err != nil {
		klog.Errorf("RcloneMounter Mount: Cannot create file %s: %v", configFileName, err)
		return err
	}
	defer file.Close()

	err = os.Chmod(configFile, 0644)
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
	datawriter.Flush()
	klog.Info("-Rclone created rclone config file-")
	return nil
}
