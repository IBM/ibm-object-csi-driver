package mounter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

type Mounter interface {
	Stage(stagePath string) error
	Unstage(stagePath string) error
	Mount(source string, target string) error
	Unmount(target string) error
}

var command = exec.Command

const (
	s3fsMounterType   = "s3fs"
	rcloneMounterType = "rclone"
)

type S3fsMounterFactory struct{}

type NewMounterFactory interface {
	// NewMounter(mounter, bucket, objpath, endpoint, region, keys string, authType string) (Mounter, error)
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
	switch mounter {
	case s3fsMounterType:
		return newS3fsMounter(secretMap, mountFlags)
	case rcloneMounterType:
		return newRcloneMounter(secretMap, mountFlags)
	default:
		// default to s3backer
		return newS3fsMounter(secretMap, mountFlags)
	}
}

func fuseMount(path string, comm string, args []string) error {
	klog.Info("-fuseMount-")
	klog.Infof("fuseMount args:\n\tpath: <%s>\n\tcommand: <%s>\n\targs: <%s>", path, comm, args)
	cmd := command(comm, args...)
	err := cmd.Start()

	if err != nil {
		klog.Errorf("fuseMount: cmd start failed: <%s>\nargs: <%s>\nerror: <%v>", comm, args, err)
		return fmt.Errorf("fuseMount: cmd start failed: <%s>\nargs: <%s>\nerror: <%v>", comm, args, err)
	}
	err = cmd.Wait()
	if err != nil {
		// Handle error
		klog.Errorf("fuseMount: cmd wait failed: <%s>\nargs: <%s>\nerror: <%v>", comm, args, err)
		return fmt.Errorf("fuseMount: cmd wait failed: <%s>\nargs: <%s>\nerror: <%v>", comm, args, err)
	}

	return waitForMount(path, 10*time.Second)
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
