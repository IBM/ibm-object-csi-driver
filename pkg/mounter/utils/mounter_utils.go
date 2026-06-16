//go:build linux
// +build linux

package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/logger"
	"github.com/mitchellh/go-ps"
	"go.uber.org/zap"
	k8sMountUtils "k8s.io/mount-utils"
)

var unmount = syscall.Unmount
var commandWithCtx = exec.CommandContext

var ErrTimeoutWaitProcess = errors.New("timeout waiting for process to end")

// Package-level logger for mounter utils
var mounterUtilLogger *zap.Logger

func init() {
	mounterUtilLogger = logger.NewJSONLoggerOrNop("cos-csi-mounter")
	mounterUtilLogger = mounterUtilLogger.With(zap.String("component", "mounter-utils"))
}

// getRequestIDFromContext extracts request_id from context
func getRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return "unknown"
	}
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		return reqID
	}
	return "unknown"
}

type MounterUtils interface {
	FuseUnmount(ctx context.Context, path string) error
	FuseMount(ctx context.Context, path string, comm string, args []string) error
}

type MounterOptsUtils struct {
}

func (su *MounterOptsUtils) FuseMount(ctx context.Context, path string, comm string, args []string) error {
	// Extract request_id from context for logging
	requestID := getRequestIDFromContext(ctx)
	log := mounterUtilLogger.With(zap.String("request_id", requestID))

	log.Info("FuseMount started")
	log.Info("FuseMount parameters",
		zap.String("path", path),
		zap.String("command", comm),
		zap.Strings("args", args))

	mountCtx, cancel := context.WithCancel(ctx)
	var mounted bool
	defer func() {
		if !mounted {
			cancel()
		}
	}()

	cmd := commandWithCtx(mountCtx, comm, args...)
	err := cmd.Start()
	if err != nil {
		log.Error("FuseMount: command start failed",
			zap.String("mounter", comm),
			zap.Strings("args", args),
			zap.Error(err))
		return fmt.Errorf("FuseMount: '%s' command start failed: %v", comm, err)
	}
	log.Info("FuseMount: command start succeeded", zap.String("mounter", comm))

	waitCh := make(chan error, 1)
	mountCh := make(chan error, 1)

	go func() {
		log.Info("FuseMount: cmd.Wait() goroutine start",
			zap.String("mounter", comm),
			zap.String("path", path))
		waitCh <- cmd.Wait()
		log.Info("FuseMount: cmd.Wait() goroutine end",
			zap.String("mounter", comm),
			zap.String("path", path))
	}()

	go func() {
		log.Info("FuseMount: waitForMount() goroutine start",
			zap.String("mounter", comm),
			zap.String("path", path))
		mountCh <- waitForMount(mountCtx, path, 2*time.Second, 30*time.Second) // kubelet retries NodePublishVolume after 120 seconds
		log.Info("FuseMount: waitForMount() goroutine end",
			zap.String("mounter", comm),
			zap.String("path", path))
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			log.Warn("FuseMount: command wait failed",
				zap.String("mounter", comm),
				zap.Strings("args", args),
				zap.Error(err))
			log.Info("FuseMount: checking if path already exists and is a mountpoint",
				zap.String("path", path))
			if isMount, err1 := isMountpoint(path); err1 == nil && isMount { // check if bucket already got mounted
				log.Info("Bucket is already mounted", zap.String("mounter", comm))
				mounted = true
				return nil
			}
			return fmt.Errorf("'%s' mount failed: %v", comm, err)
		}
		log.Info("FuseMount: command wait succeeded", zap.String("mounter", comm))
		if err := <-mountCh; err != nil {
			return err
		}

	case err := <-mountCh:
		if err != nil {
			log.Error("FuseMount: path is not mountpoint, mount failed",
				zap.String("mounter", comm),
				zap.String("path", path))
			return fmt.Errorf("'%s' mount failed: %v", comm, err)
		}
	}

	log.Info("Bucket mounted successfully", zap.String("mounter", comm))
	mounted = true
	return nil
}

func (su *MounterOptsUtils) FuseUnmount(ctx context.Context, path string) error {
	// Extract request_id from context for logging
	requestID := getRequestIDFromContext(ctx)
	log := mounterUtilLogger.With(zap.String("request_id", requestID))

	log.Info("FuseUnmount started", zap.String("path", path))
	// check if mountpoint exists
	isMount, checkMountErr := isMountpoint(path)
	if isMount || checkMountErr != nil {
		log.Info("isMountpoint check", zap.Bool("is_mount", isMount))
		err := unmount(path, 0)
		if err != nil {
			log.Warn("Standard unmount failed, trying lazy unmount",
				zap.String("path", path),
				zap.Error(err))
			// Try lazy (MNT_DETACH) unmount
			err = unmount(path, syscall.MNT_DETACH)
			if err != nil {
				log.Warn("Lazy unmount failed, trying force unmount",
					zap.String("path", path),
					zap.Error(err))
				// Try force unmount as last resort
				err = unmount(path, syscall.MNT_FORCE)
				if err != nil {
					log.Error("Force unmount failed",
						zap.String("path", path),
						zap.Error(err))
					return fmt.Errorf("all unmount attempts failed for %s: %v", path, err)
				}
				log.Info("Force unmounted successfully", zap.String("path", path))
			} else {
				log.Info("Lazy unmounted successfully", zap.String("path", path))
			}
		} else {
			log.Info("Unmounted with standard unmount successfully", zap.String("path", path))
		}
	}

	// as fuse quits immediately, we will try to wait until the process is done
	process, err := findFuseMountProcess(path)
	if err != nil {
		log.Info("Error getting PID of fuse mount", zap.Error(err))
		return nil
	}
	if process == nil {
		log.Info("Unable to find PID of fuse mount, it must have finished already",
			zap.String("path", path))
		return nil
	}
	log.Info("Found fuse pid of mount, checking if it still runs",
		zap.Int("pid", process.Pid),
		zap.String("path", path))

	err = waitForProcess(process, 1)
	if errors.Is(err, ErrTimeoutWaitProcess) {
		log.Info("Timeout waiting for pid to end, killing process",
			zap.Int("pid", process.Pid))
		return process.Kill()
	}

	return err
}

func isMountpoint(pathname string) (bool, error) {
	mounterUtilLogger.Info("Checking if path is mountpoint", zap.String("pathname", pathname))

	out, err := exec.Command("mountpoint", pathname).CombinedOutput()
	outStr := strings.ToLower(strings.TrimSpace(string(out)))
	mounterUtilLogger.Info("Mountpoint status",
		zap.String("path", pathname),
		zap.Error(err),
		zap.String("output", string(out)))
	if err != nil {
		if strings.HasSuffix(outStr, "transport endpoint is not connected") {
			return true, nil
		}
		if strings.HasSuffix(outStr, "is not a mountpoint") {
			mounterUtilLogger.Info("Path is NOT a mountpoint", zap.String("pathname", pathname))
			return false, nil
		}
		mounterUtilLogger.Error("Failed to check mountpoint for path",
			zap.String("path", pathname),
			zap.Error(err),
			zap.String("output", string(out)))
		return false, fmt.Errorf("failed to check mountpoint for path '%s', error: %v, output: %s", pathname, err, string(out))
	}
	if strings.HasSuffix(outStr, "is a mountpoint") {
		mounterUtilLogger.Info("Path is a mountpoint", zap.String("pathname", pathname))
		return true, nil
	}

	return false, nil
}

func waitForMount(ctx context.Context, path string, initialDelay, timeout time.Duration) error {
	if initialDelay > 0 {
		time.Sleep(initialDelay)
	}
	var elapsed time.Duration
	attempt := 1
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			mounterUtilLogger.Info("waitForMount: context is done", zap.Error(err))
			return err
		default:
			isMount, err := k8sMountUtils.New("").IsMountPoint(path)
			if err == nil && isMount {
				mounterUtilLogger.Info("Path is a mountpoint", zap.String("pathname", path))
				return nil
			}

			mounterUtilLogger.Info("Mountpoint check in progress",
				zap.Int("attempt", attempt),
				zap.String("path", path),
				zap.Bool("is_mount", isMount),
				zap.Error(err),
				zap.Duration("timeout", timeout))
			time.Sleep(constants.Interval)
			elapsed += constants.Interval
			if elapsed >= timeout {
				return fmt.Errorf("timeout waiting for mount. Last check response: isMount=%v, err=%v, timeout=%v", isMount, err, constants.Timeout)
			}
			attempt++
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
			mounterUtilLogger.Error("Unable to get cmdline of PID",
				zap.Int("pid", p.Pid()),
				zap.Error(err))
			continue
		}
		if strings.Contains(cmdLine, path) {
			mounterUtilLogger.Info("Found matching pid on path",
				zap.Int("pid", p.Pid()),
				zap.String("path", path))
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
		mounterUtilLogger.Warn("Error checking cmdline of PID, assuming it is dead",
			zap.Int("pid", p.Pid),
			zap.Error(err))
		return nil
	}
	if cmdLine == "" {
		// ignore defunct processes
		// TODO: debug why this happens in the first place
		// seems to only happen on k8s, not on local docker
		mounterUtilLogger.Warn("Fuse process seems dead, returning")
		return nil
	}
	if err := p.Signal(syscall.Signal(0)); err != nil {
		mounterUtilLogger.Warn("Fuse process does not seem active or we are unprivileged",
			zap.Error(err))
		return nil
	}
	mounterUtilLogger.Info("Fuse process still active, waiting",
		zap.Int("pid", p.Pid),
		zap.Int("backoff", backoff))
	time.Sleep(time.Duration(500) * time.Millisecond)
	return waitForProcess(p, backoff+1)
}
