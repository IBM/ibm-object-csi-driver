package mounter

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

var (
	mountWorker    = true
	mounterRequest = createCOSCSIMounterRequest
)

type Mounter interface {
	Mount(source string, target string) error
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

	if secretMap == nil {
		secretMap = map[string]string{}
	}
	if mountFlags == nil {
		mountFlags = []string{}
	}

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

func createCOSCSIMounterRequest(payload string, url string) (string, error) {
	// Get socket path
	socketPath := os.Getenv(constants.COSCSIMounterSocketPathEnv)
	if socketPath == "" {
		socketPath = constants.COSCSIMounterSocketPath
	}
	klog.Infof("COS CSI Mounter Socket Path: %s", socketPath)

	err := isGRPCServerAvailable(socketPath)
	if err != nil {
		return "", err
	}

	// Create a custom dialer function for Unix socket connection
	dialer := func(_ context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}

	// Create an HTTP client with the Unix socket transport
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer,
		},
		Timeout: constants.Timeout,
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

	defer func() {
		if err := response.Body.Close(); err != nil {
			klog.Errorf("failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	responseBody := string(body)
	klog.Infof("response from cos-csi-mounter -> Response body: %s, Response code: %v", responseBody, response.StatusCode)

	if response.StatusCode != http.StatusOK {
		return responseBody, fetchGRPCReturnCode(response.StatusCode)
	}
	return "", nil
}

func fetchGRPCReturnCode(code int) error {
	switch code {
	case http.StatusBadRequest:
		return status.Error(codes.InvalidArgument, "Invalid Argument")
	case http.StatusNotFound:
		return status.Error(codes.NotFound, "Not Found")
	case http.StatusConflict:
		return status.Error(codes.AlreadyExists, "Already Exists")
	case http.StatusForbidden:
		return status.Error(codes.PermissionDenied, "Permission Denied")
	case http.StatusTooManyRequests:
		return status.Error(codes.ResourceExhausted, "Resource Exhausted")
	case http.StatusNotImplemented:
		return status.Error(codes.Unimplemented, "Unimplemented")
	case http.StatusInternalServerError:
		return status.Error(codes.Internal, "Internal")
	case http.StatusServiceUnavailable:
		return status.Error(codes.Unavailable, "Unavailable")
	case http.StatusUnauthorized:
		return status.Error(codes.Unauthenticated, "Unauthenticated")
	default:
		return status.Error(codes.Unknown, "Unknown")
	}
}

// isGRPCServerAvailable tries to connect to the UNIX socket to see if it's up
func isGRPCServerAvailable(socketPath string) error {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return err
	}
	err = conn.Close()
	if err != nil {
		return err
	}
	return nil
}
