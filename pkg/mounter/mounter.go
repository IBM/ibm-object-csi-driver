//go:build linux
// +build linux

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
	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	mountWorker = true
	// mounterRequest is a function variable that can be overridden for testing
	mounterRequest = func(ctx context.Context, payload string, url string, log *zap.Logger) error {
		return createCOSCSIMounterRequest(ctx, payload, url, log)
	}

	MakeDir    = os.MkdirAll
	CreateFile = os.Create
	Chmod      = os.Chmod
	Stat       = os.Stat
	RemoveAll  = os.RemoveAll
)

type Mounter interface {
	Mount(ctx context.Context, source string, target string) error
	Unmount(ctx context.Context, target string) error
}

type CSIMounterFactory struct{}

type NewMounterFactory interface {
	NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string, defaultMOMap map[string]string) Mounter
}

func NewCSIMounterFactory() *CSIMounterFactory {
	return &CSIMounterFactory{}
}

func (s *CSIMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string, defaultMOMap map[string]string) Mounter {
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
		return NewS3fsMounter(secretMap, mountFlags, mounterUtils, defaultMOMap)
	case constants.RClone:
		return NewRcloneMounter(secretMap, mountFlags, mounterUtils)
	default:
		// default to s3fs
		return NewS3fsMounter(secretMap, mountFlags, mounterUtils, defaultMOMap)
	}
}

func checkPath(path string) (bool, error) {
	if path == "" {
		return false, errors.New("undefined path")
	}
	_, err := Stat(path)
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

func createCOSCSIMounterRequest(ctx context.Context, payload string, url string, log *zap.Logger) error {
	reqID := requestid.FromContext(ctx)

	// Get socket path
	socketPath := os.Getenv(constants.COSCSIMounterSocketPathEnv)
	if socketPath == "" {
		socketPath = constants.COSCSIMounterSocketPath
	}
	log.Info(fmt.Sprintf("[%s] COS CSI Mounter Socket Path", reqID), zap.String("socket_path", socketPath))

	err := isGRPCServerAvailable(socketPath)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] COS CSI Mounter service not available", reqID), zap.Error(err))
		return err
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
		log.Error(fmt.Sprintf("[%s] Failed to create HTTP request", reqID), zap.Error(err))
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", reqID) // Add request ID to HTTP header

	log.Info(fmt.Sprintf("[%s] Sending request to cos-csi-mounter", reqID), zap.String("url", url))
	response, err := client.Do(req)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] Failed to send request to cos-csi-mounter", reqID), zap.Error(err))
		return err
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Error(fmt.Sprintf("[%s] Failed to close response body", reqID), zap.Error(err))
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error(fmt.Sprintf("[%s] Failed to read response body", reqID), zap.Error(err))
		return err
	}

	responseBody := string(body)
	log.Info(fmt.Sprintf("[%s] Response from cos-csi-mounter", reqID),
		zap.String("response_body", responseBody),
		zap.Int("status_code", response.StatusCode))

	if response.StatusCode != http.StatusOK {
		return parseGRPCResponse(reqID, response.StatusCode, responseBody)
	}
	return nil
}

// parseGRPCResponse takes both response body and error code and frames error message
func parseGRPCResponse(reqID string, code int, response string) error {
	errMsg := parseErrFromResponse(response)
	errMsgWithReqID := fmt.Sprintf("[%s] %s", reqID, errMsg)
	switch code {
	case http.StatusBadRequest:
		return status.Error(codes.InvalidArgument, errMsgWithReqID)
	case http.StatusNotFound:
		return status.Error(codes.NotFound, errMsgWithReqID)
	case http.StatusConflict:
		return status.Error(codes.AlreadyExists, errMsgWithReqID)
	case http.StatusForbidden:
		return status.Error(codes.PermissionDenied, errMsgWithReqID)
	case http.StatusTooManyRequests:
		return status.Error(codes.ResourceExhausted, errMsgWithReqID)
	case http.StatusNotImplemented:
		return status.Error(codes.Unimplemented, errMsgWithReqID)
	case http.StatusInternalServerError:
		return status.Error(codes.Internal, errMsgWithReqID)
	case http.StatusServiceUnavailable:
		return status.Error(codes.Unavailable, errMsgWithReqID)
	case http.StatusUnauthorized:
		return status.Error(codes.Unauthenticated, errMsgWithReqID)
	default:
		return status.Error(codes.Unknown, errMsgWithReqID)
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

// parseErrFromResponse fetches error from responseBody
// e.g. ResponseBody: {"error":"invalid args for mounter: invalid s3fs args decode error: json: unknown field \"unknownkey\""}
// parseErrFromResponse returns "invalid args for mounter: invalid s3fs args decode error: json: unknown field \"unknownkey\"
func parseErrFromResponse(response string) string {
	var errFromResp map[string]string
	err := json.Unmarshal([]byte(response), &errFromResp)
	if err != nil {
		// Can't log here as we don't have logger context, just return the response
		return response
	}
	val, exists := errFromResp["error"]
	if !exists {
		return response
	}
	return val
}
