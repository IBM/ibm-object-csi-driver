package mounter

import (
	"fmt"
	"os/exec"

	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
)

type Mounter interface {
	Stage(stagePath string) error
	Unstage(stagePath string) error
	Mount(source string, target string) error
	Unmount(target string) error
}

var (
	command = exec.Command
)

const (
	s3fsMounterType   = "s3fs"
	goofysMounterType = "goofys"
	rcloneMounterType = "rclone"
	mounterTypeKey    = "mounter"
)

//func newS3fsMounter(bucket string, objpath string, endpoint string, region string, keys string)
func NewMounter(mounter string, bucket string, objpath string, endpoint string, region string, keys string) (Mounter, error) {
	klog.Info("-NewMounter-")
	klog.Infof("NewMounter args:\n\tmounter: <%s>\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>", mounter, bucket, objpath, endpoint, region)
	switch mounter {
	case s3fsMounterType:
		return newS3fsMounter(bucket, objpath, endpoint, region, keys)
	default:
		// default to s3backer
		return newS3fsMounter(bucket, objpath, endpoint, region, keys)
	}
}

func fuseMount(path string, comm string, args []string) error {
	klog.Info("-fuseMount-")
	klog.Infof("fuseMount args:\n\tpath: <%s>\n\tcommand: <%s>\n\targs: <%s>", path, comm, args)
	out, err := command(comm, args...).CombinedOutput()

	if err != nil {
		klog.Infof("fuseMount: cmd failed: <%s>\nargs: <%s>\noutput: <%s>", comm, args, out)
		return fmt.Errorf("fuseMount: cmd failed: %s\nargs: %s\noutput: %s", comm, args, out)
	}

	// TODO: need to debug waitForMount; disabling it for time being - https://github.com/IBM/satellite-object-storage-plugin/issues/45
	return nil

	// return waitForMount(path, 10*time.Second)
}

func FuseUnmount(path string) error {
	if err := mount.New("").Unmount(path); err != nil {
		return err
	}
	// as fuse quits immediately, we will try to wait until the process is done
	process, err := findFuseMountProcess(path)
	if err != nil {
		klog.Infof("Error getting PID of fuse mount: %s", err)
		return nil
	}
	if process == nil {
		klog.Infof("Unable to find PID of fuse mount %s, it must have finished already", path)
		return nil
	}
	klog.Infof("Found fuse pid %v of mount %s, checking if it still runs", process.Pid, path)
	return waitForProcess(process, 1)
}
