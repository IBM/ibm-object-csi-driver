package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/mitchellh/go-ps"
	"k8s.io/klog/v2"
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
	klog.Infof("FuseMount params:\n\tpath: <%s>\n\tcommand: <%s>\n\targs: <%s>", path, comm, args)
	out, err := command(comm, args...).CombinedOutput()
	if err != nil {
		if mounted, err1 := isMountpoint(path); err1 == nil && mounted { // check if bucket already got mounted
			klog.Infof("bucket is already mounted using '%s' mounter", comm)
			return nil
		}
		klog.Errorf("FuseMount: command execution failed: <%s>\nargs: <%s>\nerror: <%v>\noutput: <%v>", comm, args, err, string(out))
		return fmt.Errorf("'%s' mount failed: <%v>", comm, string(out))
	}
	klog.Infof("bucket mounted successfully using '%s' mounter", comm)
	return waitForMount(path, 10*time.Second)
}

func (su *MounterOptsUtils) FuseUnmount(path string) error {
	klog.Info("-fuseUnmount-")
	// directory exists
	isMount, checkMountErr := isMountpoint(path)
	if isMount || checkMountErr != nil {
		klog.Infof("isMountpoint  %v", isMount)
		err := unmount(path, 0)
		if err != nil && checkMountErr == nil {
			klog.Errorf("Cannot unmount. Trying force unmount %s", err)
			//Do force unmount
			err = unmount(path, 0)
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
	if err != nil {
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

	if strings.HasSuffix(outStr, "transport endpoint is not connected") {
		return true, nil
	}

	return false, nil
}

func waitForMount(path string, timeout time.Duration) error {
	var elapsed time.Duration
	var interval = 500 * time.Millisecond
	for {
		out, err := exec.Command("mountpoint", path).CombinedOutput()
		outStr := strings.TrimSpace(string(out))
		if err != nil {
			klog.Errorf("Failed to check mountpoint for path '%s', error: %v, output: %s", path, err, outStr)
			return err
		}
		if strings.HasSuffix(outStr, "is a mountpoint") {
			klog.Infof("Path is a mountpoint: pathname - %s", path)
			return nil
		}

		time.Sleep(interval)
		elapsed = elapsed + interval
		if elapsed >= timeout {
			return errors.New("timeout waiting for mount")
		}
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
