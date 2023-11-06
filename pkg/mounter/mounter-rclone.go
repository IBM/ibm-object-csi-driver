
//Package mounter
package mounter

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"path"
	"strings"
)

// Mounter interface defined in mounter.go
// rcloneMounter Implements Mounter
type rcloneMounter struct {
	bucketName    string //From Secret in SC
	objPath       string //From Secret in SC
	endPoint      string //From Secret in SC
	locConstraint string //From Secret in SC
	authType      string
	accessKeys    string
	mountOptions  []string
}

func updateMountOptions(dafaultMountOptions []string, secretMap map[string]string) ([]string, error) {
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
		return dafaultMountOptions, nil
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

	klog.Infof("Updated Options: %v", updatedOptions)

	return updatedOptions, nil
}

func newRcloneMounter(secretMap map[string]string, mountOptions []string) (Mounter, error) {
	klog.Info("-newRcloneMounter-")

	var (
		val       string
		check     bool
		accessKey string
		secretKey string
		apiKey    string
		mounter   *rcloneMounter
	)

	mounter = &rcloneMounter{}

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
	if apiKey != "" {
		mounter.accessKeys = fmt.Sprintf(":%s", apiKey)
		mounter.authType = "iam"
	} else {
		mounter.accessKeys = fmt.Sprintf("%s:%s", accessKey, secretKey)
		mounter.authType = "hmac"
	}

	klog.Infof("newRcloneMounter args:\n\tbucketName: [%s]\n\tobjPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.bucketName, mounter.objPath, mounter.endPoint, mounter.locConstraint, mounter.authType)

	updatedOptions, err := updateMountOptions(mountOptions, secretMap)

	if err != nil {
		klog.Infof("Problems with retrieving secret map dynamically %v", err)
	}
	mounter.mountOptions = updatedOptions

	return mounter, nil
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
		if err = os.MkdirAll(metaPath, 0755); err != nil { // #nosec G301: Required for rclone
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
		"--log-file=/var/log/rclone.log",
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
	var accessKey string
	var secretKey string
	keys := strings.Split(rclone.accessKeys, ":")
	if len(keys) == 2 {
		accessKey = keys[0]
		secretKey = keys[1]
	}
	configParams := []string{
		"[" + remote + "]",
		"type = " + s3Type,
		"endpoint = " + rclone.endPoint,
		"provider = " + cosProvider,
		"env_auth = " + envAuth,
		"location_constraint = " + rclone.locConstraint,
		"access_key_id = " + accessKey,
		"secret_access_key = " + secretKey,
	}

	configParams = append(configParams, rclone.mountOptions...)

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
	defer func() {
    		if err := file.Close(); err != nil {
        	klog.Errorf("RcloneMounter Mount: Cannot close file %s: %v", configFileName, err)
		return err
    		}
	}()

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
