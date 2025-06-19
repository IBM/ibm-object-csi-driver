package mounter

import (
	"errors"
	"fmt"
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

func Test_RcloneMount_ErrorMount(t *testing.T) {
	mounter := NewRcloneMounter(secretMapRClone, mountOptionsRClone,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return errors.New("error mounting volume")
			},
		}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Fatal("NewRCloneMounter() did not return a RCloneMounter")
	}

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	mockcreateConfig := func(configPathWithVolID string, rclone *RcloneMounter) error {
		return nil
	}

	// Replace createConfigFunc with the mock function
	createConfigFunc = mockcreateConfig
	defer func() { createConfigFunc = createConfig }()

	target := "/tmp/test-mount"

	err := rCloneMounter.Mount("source", target)
	assert.Error(t, err, "error mounting volume")
}

/*
func Test_RcloneUnmount_Positive(t *testing.T) {
	secretMap := map[string]string{
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
	mounter := NewRcloneMounter(secretMap, []string{"mountOption1", "mountOption2"},
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Fatal("NewRCloneMounter() did not return a RCloneMounter")
	}

	target := "/tmp/test-unmount"

	// Creating a directory to simulate a mounted path
	err := os.MkdirAll(target, os.ModePerm)
	if err != nil {
		t.Fatalf("TestRCloneMounter_Unmount() failed to create directory: %v", err)
	}

	err = rCloneMounter.Unmount(target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("TestRCloneMounter_Unmount() failed to unmount: %v", err)
	}

	err = os.RemoveAll(target)
	if err != nil {
		t.Errorf("Failed to remove directory: %v", err)
	}
}
*/

func Test_RcloneUnmount_Error(t *testing.T) {
	secretMap := map[string]string{
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
	mounter := NewRcloneMounter(secretMap, []string{"mountOption1", "mountOption2"},
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return errors.New("error unmounting volume")
			},
		}))

	rCloneMounter := mounter.(*RcloneMounter)

	target := "/tmp/test-unmount"

	// Creating a directory to simulate a mounted path
	err := os.MkdirAll(target, os.ModePerm)
	if err != nil {
		t.Fatalf("TestRCloneMounter_Unmount() failed to create directory: %v", err)
	}

	err = rCloneMounter.Unmount(target)
	assert.Error(t, err, "error unmounting volume")

	err = os.RemoveAll(target)
	if err != nil {
		t.Errorf("Failed to remove directory: %v", err)
	}
}

func TestUpdateRCloneMountOptions(t *testing.T) {
	defaultMountOp := []string{"option1=value1", "option2=value2"}
	secretMap := map[string]string{
		"mountOptions": "additional_option=value3",
	}

	updatedOptions := updateMountOptions(defaultMountOp, secretMap)

	assert.ElementsMatch(t, updatedOptions, []string{
		"option1=value1",
		"option2=value2",
		"additional_option=value3",
	})
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
		return fmt.Errorf("mkdir failed")
	}
	err := createConfig("/tmp/testconfig", &RcloneMounter{})
	assert.ErrorContains(t, err, "mkdir failed")
}

func TestCreateConfig_FileCreateFails(t *testing.T) {
	MakeDir = func(string, os.FileMode) error { return nil }
	CreateFile = func(string) (*os.File, error) {
		return nil, fmt.Errorf("file create failed")
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
		return fmt.Errorf("chmod failed")
	}
	err := createConfig("/tmp/testconfig", &RcloneMounter{})
	assert.ErrorContains(t, err, "chmod failed")
}
