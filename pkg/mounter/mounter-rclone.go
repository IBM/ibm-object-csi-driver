package mounter

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path"

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
	passFileRclone = ".passwd-rclone"
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
		return fmt.Errorf("RcloneMounter Mount: Cannot stat directory %s: %v", metaPath, err)
	}

	if !pathExist {
		if err = os.MkdirAll(metaPath, 0755); err != nil {
			klog.Errorf("RcloneMounter Mount: Cannot create directory %s: %v", metaPath, err)
			return fmt.Errorf("RcloneMounter Mount: Cannot create directory %s: %v", metaPath, err)
		}
	}

	//TODO: We need to see if we want to create a file to store Rclone creds or provide them some other way. For now we are mirroring s3fs set up.

	passwdFile := path.Join(metaPath, passFileRclone)
	if err = writePass(passwdFile, rclone.accessKeys); err != nil {
		klog.Errorf("rcloneMounter Mount: Cannot create file %s: %v", passwdFile, err)
		return fmt.Errorf("rcloneMounter Mount: Cannot create file %s: %v", passwdFile, err)
	}

	if rclone.objPath != "" {
		bucketName = fmt.Sprintf("%s:/%s", rclone.bucketName, rclone.objPath)
	} else {
		bucketName = fmt.Sprintf("%s", rclone.bucketName)
	}

	args := []string{
		bucketName,
		fmt.Sprintf("%s", target),
		"-o", "sigv2",
		"-o", "use_path_request_style",
		"-o", fmt.Sprintf("passwd_file=%s", passwdFile),
		"-o", fmt.Sprintf("url=%s", rclone.endPoint),
		"-o", fmt.Sprintf("endpoint=%s", rclone.regnClass),
		"-o", "allow_other",
		"-o", "mp_umask=002",
	}
	return fuseMount(target, rcloneCmd, args)
}
func (rclone *rcloneMounter) Unmount(target string) error {
	klog.Info("-RcloneMounter Unmount-")
	metaPath := path.Join(metaRootRclone, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	os.RemoveAll(metaPath)

	return FuseUnmount(target)
}

//We might want to use rclone env variables will need to have a method to set them up
// $ export RCLONE_CONFIG_OCI_TYPE=s3
// $ export RCLONE_CONFIG_OCI_ACCESS_KEY_ID=<your_access_key>
// $ export RCLONE_CONFIG_OCI_SECRET_ACCESS_KEY=<your_secret_key>
// $ export RCLONE_CONFIG_OCI_REGION=<your_region_identifier>
// $ export RCLONE_CONFIG_OCI_ENDPOINT=
