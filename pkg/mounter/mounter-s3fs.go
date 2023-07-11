package mounter

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path"

	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type s3fsMounter struct {
	bucketName string //From Secret in SC
	objPath    string //From Secret in SC
	endPoint   string //From Secret in SC
	regnClass  string //From Secret in SC
	accessKeys string
	authType string
}

const (
	s3fsCmd  = "s3fs"
	metaRoot = "/var/lib/ibmc-s3fs"
	passFile = ".passwd-s3fs"
	// defaultIAMEndPoint is the default URL of the IBM IAM endpoint
	defaultIAMEndPoint = "https://iam.cloud.ibm.com"
)

func newS3fsMounter(bucket string, objpath string, endpoint string, region string, keys string, authType string) (Mounter, error) {
	klog.Info("-newS3fsMounter-")
	klog.Infof("newS3fsMounter args:\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>\n\tkeys: <%s>", bucket, objpath, endpoint, region, keys)
	return &s3fsMounter{
		bucketName: bucket,
		objPath:    objpath,
		endPoint:   endpoint,
		regnClass:  region,
		authType:   authType,
		accessKeys: keys,
	}, nil
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
		if err = os.MkdirAll(metaPath, 0755); err != nil {
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
		"-o", fmt.Sprintf("endpoint=%s", s3fs.regnClass),
		"-o", "allow_other",
		"-o", "mp_umask=002",
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
	os.RemoveAll(metaPath)

	return FuseUnmount(target)
}
