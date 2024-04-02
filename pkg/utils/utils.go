package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/mitchellh/go-ps"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/fs"
)

var unmount = syscall.Unmount

type StatsUtils interface {
	FSInfo(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMount(targetPath string) (bool, error)
	FuseUnmount(path string) error
}

type VolumeStatsUtils struct {
}

func (su *VolumeStatsUtils) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	return fs.Info(path)
}

func (su *VolumeStatsUtils) CheckMount(targetPath string) (bool, error) {
	out, err := exec.Command("mountpoint", targetPath).CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	notMnt := true
	if err != nil {
		klog.V(3).Infof("Output: Output string error %+v", outStr)
		if strings.HasSuffix(outStr, "No such file or directory") {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				klog.V(2).Infof("checkMount: Error: %+v", err)
				return false, err
			}
			notMnt = true
		} else {
			return false, err
		}
	}
	return notMnt, nil
}

func (su *VolumeStatsUtils) FuseUnmount(path string) error {
	// directory exists
	isMount, checkMountErr := isMountpoint(path)
	if isMount || checkMountErr != nil {
		klog.Infof("isMountpoint  %v", isMount)
		err := unmount(path, syscall.MNT_DETACH)
		if err != nil && checkMountErr == nil {
			klog.Errorf("Cannot unmount. Trying force unmount %s", err)
			//Do force unmount
			err = unmount(path, syscall.MNT_FORCE)
			if err != nil {
				klog.Errorf("Cannot force unmount %s", err)
				return fmt.Errorf("cannot force unmount %s: %v", path, err)
			}
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
	return waitForProcess(process, 1)
}

func isMountpoint(pathname string) (bool, error) {
	klog.Infof("Checking if path is mountpoint: Pathname - %s", pathname)

	out, err := exec.Command("mountpoint", pathname).CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		if strings.HasSuffix(outStr, "Transport endpoint is not connected") {
			return true, err
		} else {
			return false, err
		}
	}

	if strings.HasSuffix(outStr, "is a mountpoint") {
		klog.Infof("Path is a mountpoint: pathname - %s", pathname)
		return true, nil
	} else if strings.HasSuffix(outStr, "is not a mountpoint") {
		klog.Infof("Path is NOT a mountpoint:Pathname - %s", pathname)
		return false, nil
	} else {
		klog.Errorf("Cannot parse mountpoint result: %v", outStr)
		return false, fmt.Errorf("cannot parse mountpoint result: %s", outStr)
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
	if backoff == 20 {
		return fmt.Errorf("timeout waiting for PID %v to end", p.Pid)
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
	klog.Infof("Fuse process with PID %v still active, waiting...", p.Pid)
	time.Sleep(time.Duration(backoff*100) * time.Millisecond)
	return waitForProcess(p, backoff+1)
}
