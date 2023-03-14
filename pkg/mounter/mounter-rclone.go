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
	bucketName string //From Secret in SC
	objPath    string //From Secret in SC
	endPoint   string //From Secret in SC
	regnClass  string //From Secret in SC
	accessKeys string
}

func newRcloneMounter(bucket string, objpath string, endpoint string, region string, keys string) (Mounter, error) {
	klog.Info("-newRcloneMounter-")
	klog.Infof("newRcloneMounter args:\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>\n\tkeys: <%s>", bucket, objpath, endpoint, region, keys)
	return &rcloneMounter{
		bucketName: bucket,
		objPath:    objpath,
		endPoint:   endpoint,
		regnClass:  region,
		accessKeys: keys,
	}, nil
}

const (
	rcloneCmd      = "rclone"
	metaRootRclone = "/var/lib/ibmc-rclone"
	configPath     = "/root/.config/rclone"
	configFileName = "rclone.conf"
	remote         = "rclone-remote"
	s3Type         = "s3"
	provider       = "IBMCOS"
	env_auth       = "true"
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

	if err = createConfig(rclone.endPoint, rclone.regnClass, rclone.accessKeys); err != nil {
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

func createConfig(endpoint, location_constraint, accessKeys string) error {
	keys := strings.Split(accessKeys, ":")
	lines := []string{
		"[" + remote + "]",
		"type = " + s3Type,
		"endpoint = " + endpoint,
		"provider = " + provider,
		"env_auth = " + env_auth,
		"location_constraint = " + location_constraint,
		"access_key_id = " + keys[0],
		"secret_access_key = " + keys[1],
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
	for _, line := range lines {
		_, err = datawriter.WriteString(line + "\n")
		if err != nil {
			klog.Errorf("RcloneMounter Mount: Could not write file: %v", err)
			return err
		}
	}
	datawriter.Flush()
	klog.Info("-Rclone created rclone config file-")
	return nil
}
