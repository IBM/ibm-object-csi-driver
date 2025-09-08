//go:build linux
// +build linux

package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/mitchellh/go-ps"
	"k8s.io/klog/v2"
	k8sMountUtils "k8s.io/mount-utils"
)

var unmount = syscall.Unmount
var command = exec.Command

var ErrTimeoutWaitProcess = errors.New("timeout waiting for process to end")

type MounterUtils interface {
	FuseUnmount(path string) error
	FuseMount(path string, comm string, args []string) error
}

type MounterOptsUtils struct {
}

func (su *MounterOptsUtils) FuseMount(path string, comm string, args []string) error {
	klog.Info("-FuseMount-")
	klog.Infof("FuseMount params:\n\tpath: <%s>\n\tcommand: <%s>\n\targs: <%v>", path, comm, args)
	out, err := command(comm, args...).CombinedOutput()
	if err != nil {
		klog.Warningf("FuseMount: mount command failed: mounter=%s, args=%v, error=%v, output=%s", comm, args, err, string(out))
		klog.Infof("FuseMount: checking if path already exists and is a mountpoint: path=%s", path)
		if mounted, err1 := isMountpoint(path); err1 == nil && mounted { // check if bucket already got mounted
			klog.Infof("bucket is already mounted using '%s' mounter", comm)
			return nil
		}
		klog.Errorf("FuseMount: path is not mountpoint, mount failed: path=%s", path)
		return fmt.Errorf("'%s' mount failed: <%v>", comm, string(out))
	}
	klog.Infof("mount command succeeded: mounter=%s, output=%s", comm, string(out))
	if err := waitForMount(path, 10*time.Second); err != nil {
		return err
	}
	klog.Infof("bucket mounted successfully using '%s' mounter", comm)
	return nil
}

func (su *MounterOptsUtils) FuseUnmount(path string) error {
	klog.Info("-fuseUnmount-")
	// check if mountpoint exists
	isMount, checkMountErr := isMountpoint(path)
	if checkMountErr != nil && strings.Contains(strings.ToLower(checkMountErr.Error()), "is not mountpoint") {
		klog.Infof("isMountpoint returned 'is not mountpoint' error, skipping unmount.")
	} else if isMount || checkMountErr != nil {
		klog.Infof("isMountpoint  %v", isMount)
		err := unmount(path, 0)
		if err != nil {
			klog.Warningf("Standard unmount failed for %s: %v. Trying lazy unmount...", path, err)
			// Try lazy (MNT_DETACH) unmount
			err = unmount(path, syscall.MNT_DETACH)
			if err != nil {
				klog.Warningf("Lazy unmount failed for %s: %v. Trying force unmount...", path, err)
				// Try force unmount as last resort
				err = unmount(path, syscall.MNT_FORCE)
				if err != nil {
					klog.Errorf("Force unmount failed for %s: %v", path, err)
					return fmt.Errorf("all unmount attempts failed for %s: %v", path, err)
				}
				klog.Infof("Force unmounted %s successfully", path)
			} else {
				klog.Infof("Lazy unmounted %s successfully", path)
			}
		} else {
			klog.Infof("Unmounted %s with standard unmount successfully", path)
		}
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

	err = waitForProcess(process, 1)
	if errors.Is(err, ErrTimeoutWaitProcess) {
		klog.Infof("timeout waiting for pid %d to end, killing process", process.Pid)
		return process.Kill()
	}

	return err
}

func isMountpoint(pathname string) (bool, error) {
	klog.Infof("Checking if path is mountpoint: Pathname - %s", pathname)

	out, err := exec.Command("mountpoint", pathname).CombinedOutput()
	outStr := strings.ToLower(strings.TrimSpace(string(out)))
	klog.Infof("mountpoint status for path '%s', error: %v, output: %s", pathname, err, string(out))
	if err != nil {
		if strings.HasSuffix(outStr, "transport endpoint is not connected") {
			return true, nil
		}
		klog.Errorf("Failed to check mountpoint for path '%s', error: %v, output: %s", pathname, err, string(out))
		return false, fmt.Errorf("failed to check mountpoint for path '%s', error: %v, output: %s", pathname, err, string(out))
	}
	if strings.HasSuffix(outStr, "is a mountpoint") {
		klog.Infof("Path is a mountpoint: pathname - %s", pathname)
		return true, nil
	}

	if strings.HasSuffix(outStr, "is not a mountpoint") {
		klog.Infof("Path is NOT a mountpoint: pathname - %s", pathname)
		return false, nil
	}

	return false, nil
}

func waitForMount(path string, timeout time.Duration) error {
	var elapsed time.Duration
	attempt := 1
	for {
		isMount, err := k8sMountUtils.New("").IsMountPoint(path)
		if err == nil && isMount {
			klog.Infof("Path is a mountpoint: pathname: %s", path)
			return nil
		}

		klog.Infof("Mountpoint check in progress: attempt=%d, path=%s, isMount=%v, err=%v", attempt, path, isMount, err)
		time.Sleep(constants.Interval)
		elapsed += constants.Interval
		if elapsed >= timeout {
			return fmt.Errorf("timeout waiting for mount. Last check response: isMount=%v, err=%v", isMount, err)
		}
		attempt++
	}
}

func findFuseMountProcess(path string) (*os.Process, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	for _, p := range processes {
		cmdLine, err := getCmdLine(p.Pid())
		if err != nil {
			klog.Errorf("Unable to get cmdline of PID %v: %s", p.Pid(), err)
			continue
		}
		if strings.Contains(cmdLine, path) {
			klog.Infof("Found matching pid %v on path %s", p.Pid(), path)
			return os.FindProcess(p.Pid())
		}
	}
	return nil, nil
}

func getCmdLine(pid int) (string, error) {
	cmdLineFile := fmt.Sprintf("/proc/%v/cmdline", pid)
	cmdLine, err := os.ReadFile(cmdLineFile) // #nosec G304: Dynamic pid .
	if err != nil {
		return "", err
	}
	return string(cmdLine), nil
}

func waitForProcess(p *os.Process, backoff int) error {
	// totally it waits 60 seconds before force killing the process
	if backoff == 120 {
		return ErrTimeoutWaitProcess
	}
	cmdLine, err := getCmdLine(p.Pid)
	if err != nil {
		klog.Warningf("Error checking cmdline of PID %v, assuming it is dead: %s", p.Pid, err)
		return nil
	}
	if cmdLine == "" {
		// ignore defunct processes
		// TODO: debug why this happens in the first place
		// seems to only happen on k8s, not on local docker
		klog.Warning("Fuse process seems dead, returning")
		return nil
	}
	if err := p.Signal(syscall.Signal(0)); err != nil {
		klog.Warningf("Fuse process does not seem active or we are unprivileged: %s", err)
		return nil
	}
	klog.Infof("Fuse process with PID %v still active, waiting... %v", p.Pid, backoff)
	time.Sleep(time.Duration(500) * time.Millisecond)
	return waitForProcess(p, backoff+1)
}
