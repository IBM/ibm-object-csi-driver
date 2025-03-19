package mounter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

var mountWorker bool = true

type Mounter interface {
	Mount(source string, target string, secretMap map[string]string) error
	Unmount(target string) error
}

type CSIMounterFactory struct{}

type NewMounterFactory interface {
	NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) Mounter
}

func NewCSIMounterFactory() *CSIMounterFactory {
	return &CSIMounterFactory{}
}

func (s *CSIMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) Mounter {
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
		// default to s3fs
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

var mkdirAllFunc = os.MkdirAll

// Function that wraps os.MkdirAll
var mkdirAll = func(path string, perm os.FileMode) error {
	return mkdirAllFunc(path, perm)
}

func createMountHelperContainerRequest(payload string, url string) (string, error) {
	timeout := 3 * time.Minute
	defaultSocketPath := "/tmp/mysocket.sock"

	// Get socket path
	// socketPath := os.Getenv("SOCKET_PATH")
	socketPath := "/var/lib/ibmshare.sock"
	if socketPath == "" {
		socketPath = defaultSocketPath
	}
	// Create a custom dialer function for Unix socket connection
	dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}

	// Create an HTTP client with the Unix socket transport
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer,
		},
		Timeout: timeout,
	}

	// Create POST request
	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	// Unmarshal json response
	var responseBody struct {
		MountExitCode   string `json:"MountExitCode"`
		ExitDescription string `json:"Description"`
	}
	err = json.Unmarshal(body, &responseBody)
	if err != nil {
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		return responseBody.ExitDescription, fmt.Errorf("response from mount-helper-container -> Exit Status Code: %s ,ResponseCode: %v", responseBody.MountExitCode, response.StatusCode)
	}
	return "", nil
}
