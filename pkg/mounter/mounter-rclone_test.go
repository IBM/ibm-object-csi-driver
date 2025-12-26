package mounter

import (
	"errors"
	"os"
	"testing"

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

var (
	secretMapRClone = map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "fake-gid",
		"uid":                "fake-uid",
	}

	mountOptionsRClone = []string{"opt1=val1", "opt2=val2"}
	target             = "/tmp/test-mount"
	source             = "source"
)

func TestNewRcloneMounter_Success(t *testing.T) {
	mounter := NewRcloneMounter(secretMapRClone, mountOptionsRClone, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, rCloneMounter.BucketName, secretMapRClone["bucketName"])
	assert.Equal(t, rCloneMounter.objectPath, secretMapRClone["objectPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMapRClone["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMapRClone["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMapRClone["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMapRClone["gid"])
}

func TestNewRcloneMounter_Only_GID(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "1001",
	}
	mounter := NewRcloneMounter(secretMap, mountOptionsRClone, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.objectPath, secretMap["objectPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
}

func TestNewRcloneMounter_MountOptsInSecret(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "1001",
		"uid":                "1001",
		"mountOptions":       "\nupload_concurrency\nkey=value",
	}
	mounter := NewRcloneMounter(secretMap, mountOptionsRClone, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.objectPath, secretMap["objectPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMap["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
}

func TestRcloneMount_NodeServer_Positive(t *testing.T) {
	mountWorker = false

	rclone := &RcloneMounter{
		BucketName: "testBucket",
		AccessKeys: "testAccessKey",
		EndPoint:   "testEndpoint",
		GID:        "testGID",
		UID:        "testUID",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter) error {
		return nil
	}

	err := rclone.Mount(source, target)
	assert.NoError(t, err)
}

func TestRcloneMount_CreateConfigFails_Negative(t *testing.T) {
	rclone := &RcloneMounter{}

	createConfigWrap = func(_ string, _ *RcloneMounter) error {
		return errors.New("failed to create config file")
	}

	err := rclone.Mount(source, target)
	assert.Error(t, err)
	assert.EqualError(t, err, "failed to create config file")
}

func TestRcloneMount_WorkerNode_Positive(t *testing.T) {
	mountWorker = true

	rclone := &RcloneMounter{
		BucketName: "testBucket",
		AccessKeys: "testAccessKey",
		EndPoint:   "testEndpoint",
		GID:        "testGID",
		UID:        "testUID",
		objectPath: "testObjectPath",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return nil
	}

	err := rclone.Mount(source, target)
	assert.NoError(t, err)
}

func TestRcloneMount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	rclone := &RcloneMounter{
		BucketName: "testBucket",
		AccessKeys: "testAccessKey",
		EndPoint:   "testEndpoint",
		GID:        "testGID",
		UID:        "testUID",
		objectPath: "testObjectPath",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return errors.New("failed to create http request")
	}

	err := rclone.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestRcloneUnmount_NodeServer(t *testing.T) {
	mountWorker = false

	removeConfigFile = func(_, _ string) {}

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	err := rclone.Unmount(target)
	assert.NoError(t, err)
}

func TestRcloneUnmount_WorkerNode(t *testing.T) {
	mountWorker = true

	removeConfigFile = func(_, _ string) {}

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return nil
	}

	err := rclone.Unmount(target)
	assert.NoError(t, err)
}

func TestRcloneUnmount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return errors.New("failed to create http request")
	}

	err := rclone.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestRcloneUnmount_NodeServer_Negative(t *testing.T) {
	mountWorker = false

	removeConfigFile = func(_, _ string) {}

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return errors.New("failed to unmount")
		},
	})}

	err := rclone.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmount")
}

func TestCreateConfig_Success(t *testing.T) {
	rclone := &RcloneMounter{
		AccessKeys:    "accessKey:secretKey",
		LocConstraint: "us-south",
	}

	err := createConfig("/tmp/testconfig", rclone)
	assert.Nil(t, err)
}

func TestCreateConfig_MakeDirFails(t *testing.T) {
	MakeDir = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}
	err := createConfig("/tmp/testconfig", &RcloneMounter{})
	assert.ErrorContains(t, err, "mkdir failed")
}

func TestCreateConfig_FileCreateFails(t *testing.T) {
	MakeDir = func(string, os.FileMode) error { return nil }
	CreateFile = func(string) (*os.File, error) {
		return nil, errors.New("file create failed")
	}
	err := createConfig("/tmp/testconfig", &RcloneMounter{})
	assert.ErrorContains(t, err, "file create failed")
}

func TestCreateConfig_ChmodFails(t *testing.T) {
	MakeDir = func(string, os.FileMode) error { return nil }
	CreateFile = func(string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(string, os.FileMode) error {
		return errors.New("chmod failed")
	}
	err := createConfig("/tmp/testconfig", &RcloneMounter{})
	assert.ErrorContains(t, err, "chmod failed")
}

func TestRemoveRcloneConfigFile_PathNotExists(t *testing.T) {
	Stat = func(path string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	defer func() {
		Stat = os.Stat
	}()

	removeRcloneConfigFile("/test", target)
}

func TestRemoveRcloneConfigFile_StatRetryThenSuccess(t *testing.T) {
	attempt := 0
	Stat = func(_ string) (os.FileInfo, error) {
		if attempt == 0 {
			attempt++
			return nil, errors.New("stat error")
		}
		return nil, nil
	}
	defer func() {
		Stat = os.Stat
	}()

	RemoveAll = func(_ string) error {
		return nil
	}

	removeRcloneConfigFile("/test1", target)
}

func TestRemoveRcloneConfigFile_RemoveRetryThenSuccess(t *testing.T) {
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

	defer func() {
		Stat = os.Stat
		RemoveAll = os.RemoveAll
	}()

	removeRcloneConfigFile("/test", target)
}

func TestRemoveRcloneConfigFile_Negative(t *testing.T) {
	called := 0
	Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}
	RemoveAll = func(_ string) error {
		called++
		return errors.New("remove failed")
	}

	defer func() {
		Stat = os.Stat
		RemoveAll = os.RemoveAll
	}()

	removeRcloneConfigFile("/test", target)
	assert.Equal(t, maxRetries, called)
}
