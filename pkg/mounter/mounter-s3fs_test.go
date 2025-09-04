package mounter

import (
	"errors"
	"os"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

var (
	secretMap = map[string]string{
		"cosEndpoint":             "test-endpoint",
		"locationConstraint":      "test-loc-constraint",
		"bucketName":              "test-bucket-name",
		"objPath":                 "test-obj-path",
		"accessKey":               "test-access-key",
		"secretKey":               "test-secret-key",
		"apiKey":                  "test-api-key",
		"kpRootKeyCRN":            "test-kp-root-key-crn",
		"uid":                     "test-uid",
		constants.CipherSuitesKey: "default",
	}

	mountOptions = []string{"opt1=val1", "opt2=val2", "opt3"}
)

func TestNewS3fsMounter_Success(t *testing.T) {
	mounter := NewS3fsMounter(secretMap, mountOptions, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}), map[string]string{constants.CipherSuitesKey: "default"})

	s3fsMounter, ok := mounter.(*S3fsMounter)
	assert.True(t, ok)

	assert.Equal(t, s3fsMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, s3fsMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, s3fsMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, s3fsMounter.LocConstraint, secretMap["locationConstraint"])
}

func TestNewS3fsMounter_Success_Hmac(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"mountOptions":       "\nkey1\nkey2=value2\nkey=val1=val2",
		"tmpdir":             "test-tmpdir",
		"use_cache":          "true",
		"gid":                "test-gid",
		"iamEndpoint":        "test-iamEndpoint",
	}

	mountOptions := []string{"opt1=val1", "opt2=val2", " ", "opt3"}

	mounter := NewS3fsMounter(secretMap, mountOptions, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}), nil)

	s3fsMounter, ok := mounter.(*S3fsMounter)
	assert.True(t, ok)

	assert.Equal(t, s3fsMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, s3fsMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, s3fsMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, s3fsMounter.LocConstraint, secretMap["locationConstraint"])
}

func TestS3FSMount_NodeServer_Positive(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return nil
	}

	s3fs := &S3fsMounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
		LocConstraint: "test-location",
		MountOptions:  mountOptions,
		ObjPath:       "test-objPath",
	}

	err := s3fs.Mount(source, target)
	assert.NoError(t, err)
}

func TestS3FSMount_WorkerNode_Positive(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return nil
	}

	s3fs := &S3fsMounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
		AuthType: "hmac",
	}

	err := s3fs.Mount(source, target)
	assert.NoError(t, err)
}

func TestMount_CreateDirFails_Negative(t *testing.T) {
	s3fs := &S3fsMounter{}

	MakeDir = func(path string, perm os.FileMode) error {
		return errors.New("failed to create directory")
	}

	err := s3fs.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot create directory")
}

func TestMount_FailedToCreatePassFile_Negative(t *testing.T) {
	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return errors.New("failed to create file")
	}

	s3fs := &S3fsMounter{}

	err := s3fs.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create file")
}

func TestS3FSMount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return errors.New("failed to perform http request")
	}

	s3fs := &S3fsMounter{}

	err := s3fs.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to perform http request")
}

func TestUnmount_NodeServer(t *testing.T) {
	mountWorker = false

	removeFile = func(_, _ string) {}

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	err := s3fs.Unmount(target)
	assert.NoError(t, err)
}

func TestUnmount_WorkerNode(t *testing.T) {
	mountWorker = true

	removeFile = func(_, _ string) {}

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return nil
	}

	err := s3fs.Unmount(target)
	assert.NoError(t, err)
}

func TestUnmount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return errors.New("failed to create http request")
	}

	err := s3fs.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestUnmount_NodeServer_Negative(t *testing.T) {
	mountWorker = false

	removeFile = func(_, _ string) {}

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return errors.New("failed to unmount")
		},
	})}

	err := s3fs.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmount")
}

func TestRemoveS3FSCredFile_PathNotExists(t *testing.T) {
	Stat = func(path string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	removeS3FSCredFile("/test", target)
}

func TestRemoveS3FSCredFile_StatRetryThenSuccess(t *testing.T) {
	attempt := 0
	Stat = func(_ string) (os.FileInfo, error) {
		if attempt == 0 {
			attempt++
			return nil, errors.New("stat error")
		}
		return nil, nil
	}
	RemoveAll = func(_ string) error {
		return nil
	}

	removeS3FSCredFile("/test", target)
}

func TestRemoveS3FSCredFile_RemoveRetryThenSuccess(t *testing.T) {
	Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}
	attempt := 0
	RemoveAll = func(_ string) error {
		if attempt == 0 {
			attempt++
			return errors.New("remove error")
		}
		return nil
	}

	removeS3FSCredFile("/test", target)
}

func TestRemoveS3FSCredFile_Negative(t *testing.T) {
	called := 0
	Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}
	RemoveAll = func(_ string) error {
		called++
		return errors.New("remove failed")
	}

	removeS3FSCredFile("/test", target)
	assert.Equal(t, maxRetries, called)
}
