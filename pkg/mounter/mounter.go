package mounter

import (
	"errors"
	"os"
	"syscall"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

type Mounter interface {
	Mount(source string, target string) error
	Unmount(target string) error
}

type S3fsMounterFactory struct{}

type NewMounterFactory interface {
	NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) (Mounter, error)
}

func NewS3fsMounterFactory() *S3fsMounterFactory {
	return &S3fsMounterFactory{}
}

// func newS3fsMounter(bucket string, objpath string, endpoint string, region string, keys string)
func (s *S3fsMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) (Mounter, error) {
	klog.Info("-NewMounter-")
	var mounter, val string
	var check bool

	// Select mounter as per storage class
	if val, check = attrib["mounter"]; check {
		mounter = val
	} else {
		// if mounter not set in storage class
		if val, check = secretMap["mounter"]; check {
			mounter = val
		}
	}

	mounterUtils := &(mounterUtils.MounterOptsUtils{})

	switch mounter {
	case constants.S3FS:
		return NewS3fsMounter(secretMap, mountFlags, mounterUtils)
	case constants.RClone:
		return NewRcloneMounter(secretMap, mountFlags, mounterUtils)
	default:
		// default to s3backer
		return NewS3fsMounter(secretMap, mountFlags, mounterUtils)
	}
}

func checkPath(path string) (bool, error) {
	if path == "" {
		return false, errors.New("undefined path")
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else if isCorruptedMnt(err) {
		return true, err
	}
	return false, err
}

func isCorruptedMnt(err error) bool {
	if err == nil {
		return false
	}
	var underlyingError error
	switch pe := err.(type) {
	case *os.PathError:
		underlyingError = pe.Err
	case *os.LinkError:
		underlyingError = pe.Err
	case *os.SyscallError:
		underlyingError = pe.Err
	}
	return underlyingError == syscall.ENOTCONN || underlyingError == syscall.ESTALE
}

func writePass(pwFileName string, pwFileContent string) error {
	pwFile, err := os.OpenFile(pwFileName, os.O_RDWR|os.O_CREATE, 0600) // #nosec G304: Value is dynamic
	if err != nil {
		return err
	}
	_, err = pwFile.WriteString(pwFileContent)
	if err != nil {
		return err
	}
	err = pwFile.Close() // #nosec G304: Value is dynamic
	if err != nil {
		return err
	}
	return nil
}
