package mounter

import (
	"errors"
	"testing"

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

var (
	secretMapRClone = map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
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
	assert.Equal(t, rCloneMounter.ObjPath, secretMapRClone["objPath"])
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
		"objPath":            "test-obj-path",
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
	assert.Equal(t, rCloneMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
}

func TestNewRcloneMounter_MountOptsInSecret(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
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
	assert.Equal(t, rCloneMounter.ObjPath, secretMap["objPath"])
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
		ObjPath:    "testObjPath",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter) error {
		return nil
	}
	mounterRequest = func(_, _ string) (string, error) {
		return "", nil
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
		ObjPath:    "testObjPath",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter) error {
		return nil
	}
	mounterRequest = func(_, _ string) (string, error) {
		return "", errors.New("failed to create http request")
	}

	err := rclone.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestRcloneUnmount_NodeServer(t *testing.T) {
	mountWorker = false

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

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) (string, error) {
		return "", nil
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

	mounterRequest = func(_, _ string) (string, error) {
		return "", errors.New("failed to create http request")
	}

	err := rclone.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}
