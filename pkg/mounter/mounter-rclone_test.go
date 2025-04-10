// Package mounter
package mounter

import (
	"errors"
	"os"
	"testing"

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

// Mock the secretMap and mountOptions
var secretMapRClone = map[string]string{
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

var mountOptionsRClone = []string{"opt1=val1", "opt2=val2"}

func TestNewRcloneMounter_Success(t *testing.T) {
	mounter := NewRcloneMounter(secretMapRClone, mountOptionsRClone, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Errorf("NewRCloneMounter() failed to return an instance of RCloneMounter")
	}

	assert.Equal(t, rCloneMounter.BucketName, secretMapRClone["bucketName"])
	assert.Equal(t, rCloneMounter.ObjPath, secretMapRClone["objPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMapRClone["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMapRClone["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMapRClone["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMapRClone["gid"])
}

func TestNewRcloneMounter_Success_Hmac(t *testing.T) {
	// Mock the secretMap and mountOptions
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "fake-gid",
		"uid":                "fake-uid",
	}

	mounter := NewRcloneMounter(secretMap, mountOptionsRClone, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Errorf("NewRCloneMounter() failed to return an instance of RCloneMounter")
	}

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMap["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
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
	if !ok {
		t.Errorf("NewRCloneMounter() failed to return an instance of RCloneMounter")
	}

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
}

func TestNewRcloneMounter_MountOptsInSecret_Invalid(t *testing.T) {
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
		"mountOptions":       "upload_concurrency",
	}
	mounter := NewRcloneMounter(secretMap, mountOptionsRClone, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Errorf("NewRCloneMounter() failed to return an instance of RCloneMounter")
	}

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMap["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
}

/*
func Test_RcloneMount_Positive(t *testing.T) {
	mounter := NewRcloneMounter(secretMapRClone, mountOptionsRClone,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Fatal("NewRCloneMounter() did not return a RCloneMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeCreateConfig := func(configPathWithVolID string, rclone *RcloneMounter) error {
		return nil
	}
	// Replace createConfigFunc with the mock function
	createConfigFunc = FakeCreateConfig
	defer func() { createConfigFunc = createConfig }()

	target := "/tmp/test-mount"

	err := rCloneMounter.Mount("source", target)
	assert.NoError(t, err)
}

func Test_RcloneMount_Positive_Empty_ObjPath(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "fake-gid",
		"uid":                "fake-uid",
	}
	mounter := NewRcloneMounter(secretMap, mountOptionsRClone,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Fatal("NewRCloneMounter() did not return a RCloneMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeCreateConfig := func(configPathWithVolID string, rclone *RcloneMounter) error {
		return nil
	}
	// Replace createConfigFunc with the mock function
	createConfigFunc = FakeCreateConfig
	defer func() { createConfigFunc = createConfig }()

	target := "/tmp/test-mount"

	err := rCloneMounter.Mount("source", target)
	assert.NoError(t, err)
}
*/

func Test_RcloneMount_Error_Creating_Mount_Point(t *testing.T) {
	mounter := NewRcloneMounter(secretMapRClone, mountOptionsRClone,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))

	rCloneMounter, ok := mounter.(*RcloneMounter)
	if !ok {
		t.Fatal("NewRCloneMounter() did not return a RCloneMounter")
	}

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return errors.New("error creating mount path")
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	target := "/tmp/test-mount"

	err := rCloneMounter.Mount("source", target)
	assert.Error(t, err, "Cannot create directory")
}

func Test_RcloneMount_Error_Creating_ConfigFile(t *testing.T) {
	mounter := NewRcloneMounter(secretMapRClone, mountOptionsRClone,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
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
		return errors.New("error creating ConfigFile")
	}

	// Replace createConfigFunc with the mock function
	createConfigFunc = mockcreateConfig
	defer func() { createConfigFunc = createConfig }()

	target := "/tmp/test-mount"

	err := rCloneMounter.Mount("source", target)
	assert.Error(t, err, "Cannot create file")
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
*/

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
