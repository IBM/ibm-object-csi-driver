package mounter

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"os"
	"path"
	"syscall"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type s3fsMounter struct {
	bucketName string //From Secret in SC
	objPath    string //From Secret in SC
	endPoint   string //From Secret in SC
	regnClass  string //From Secret in SC
	accessKeys string
}

const (
	s3fsCmd  = "s3fs"
	metaRoot = "/var/lib/ibmc-s3fs"
	passFile = ".passwd-s3fs"
)

func newS3fsMounter(bucket string, objpath string, endpoint string, region string, keys string) (Mounter, error) {
	glog.Infof("-newS3fsMounter-")
	glog.Infof("newS3fsMounter args:\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>\n\tkeys: <%s>", bucket, objpath, endpoint, region, keys)
	return &s3fsMounter{
		bucketName: bucket,
		objPath:    objpath,
		endPoint:   endpoint,
		regnClass:  region,
		accessKeys: keys,
	}, nil
}

func (s3fs *s3fsMounter) Stage(stageTarget string) error {
	glog.Infof("-S3FSMounter Stage-")
	return nil
}

func (s3fs *s3fsMounter) Unstage(stageTarget string) error {
	glog.Infof("-S3FSMounter Unstage-")
	return nil
}

func (s3fs *s3fsMounter) Mount(source string, target string) error {
	glog.Infof("-S3FSMounter Mount-")
	glog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)
	var bucketName string
	var pathExist bool
	var err error
	metaPath := path.Join(metaRoot, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if pathExist, err = checkPath(metaPath); err != nil {
		glog.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
		return fmt.Errorf("S3FSMounter Mount: Cannot stat directory %s: %v", metaPath, err)
	}

	if !pathExist {
		if err = os.MkdirAll(metaPath, 0755); err != nil {
			glog.Errorf("S3FSMounter Mount: Cannot create directory %s: %v", metaPath, err)
			return fmt.Errorf("S3FSMounter Mount: Cannot create directory %s: %v", metaPath, err)
		}
	}

	passwdFile := path.Join(metaPath, passFile)
	if err = writes3fsPass(passwdFile, s3fs.accessKeys); err != nil {
		glog.Errorf("S3FSMounter Mount: Cannot create file %s: %v", passwdFile, err)
		return fmt.Errorf("S3FSMounter Mount: Cannot create file %s: %v", passwdFile, err)
	}

	if s3fs.objPath != "" {
		bucketName = fmt.Sprintf("%s:/%s", s3fs.bucketName, s3fs.objPath)
	} else {
		bucketName = fmt.Sprintf("%s", s3fs.bucketName)
	}

	args := []string{
		bucketName,
		fmt.Sprintf("%s", target),
		"-o", "sigv2",
		"-o", "use_path_request_style",
		"-o", fmt.Sprintf("passwd_file=%s", passwdFile),
		"-o", fmt.Sprintf("url=%s", s3fs.endPoint),
		"-o", fmt.Sprintf("endpoint=%s", s3fs.regnClass),
		"-o", "allow_other",
		"-o", "mp_umask=002",
	}
	return fuseMount(target, s3fsCmd, args)
}

func (s3fs *s3fsMounter) Unmount(target string) error {
	glog.Infof("-S3FSMounter Unmount-")
	metaPath := path.Join(metaRoot, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	os.RemoveAll(metaPath)

	return FuseUnmount(target)
}

func checkPath(path string) (bool, error) {
	if path == "" {
		return false, errors.New("Undefined path")
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else if isCorruptedMnt(err) {
		return true, err
	} else {
		return false, err
	}
}

func isCorruptedMnt(err error) bool {
	if err == nil {
		return false
	}
	var underlyingError error
	switch pe := err.(type) {
	case nil:
		return false
	case *os.PathError:
		underlyingError = pe.Err
	case *os.LinkError:
		underlyingError = pe.Err
	case *os.SyscallError:
		underlyingError = pe.Err
	}
	return underlyingError == syscall.ENOTCONN || underlyingError == syscall.ESTALE
}

func writes3fsPass(pwFileName string, pwFileContent string) error {
	pwFile, err := os.OpenFile(pwFileName, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	_, err = pwFile.WriteString(pwFileContent)
	if err != nil {
		return err
	}
	pwFile.Close()
	return nil
}
