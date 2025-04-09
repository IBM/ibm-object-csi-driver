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
var secretMapMountpoint = map[string]string{
	"cosEndpoint": "test-endpoint",
	"bucketName":  "test-bucket-name",
	"objPath":     "test-obj-path",
	"accessKey":   "test-access-key",
	"secretKey":   "test-secret-key",
	"apiKey":      "test-api-key",
}

var mountOptionsMountpoint = []string{"opt1=val1", "opt2=val2"}

func TestNewMountpointMounter_Success(t *testing.T) {
	mounter := NewMountpointMounter(secretMapMountpoint, mountOptionsMountpoint, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Errorf("NewMountpointMounter() failed to return an instance of MountpointMounter")
	}

	assert.Equal(t, mntpMounter.BucketName, secretMapMountpoint["bucketName"])
	assert.Equal(t, mntpMounter.ObjPath, secretMapMountpoint["objPath"])
	assert.Equal(t, mntpMounter.EndPoint, secretMapMountpoint["cosEndpoint"])
}

func TestNewMountpointMounter_Success_Hmac(t *testing.T) {
	// Mock the secretMap and mountOptions
	secretMap := map[string]string{
		"cosEndpoint": "test-endpoint",
		"bucketName":  "test-bucket-name",
		"objPath":     "test-obj-path",
		"accessKey":   "test-access-key",
		"secretKey":   "test-secret-key",
	}

	mounter := NewMountpointMounter(secretMap, mountOptionsMountpoint, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Errorf("NewMountpointMounter() failed to return an instance of MountpointMounter")
	}

	assert.Equal(t, mntpMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, mntpMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, mntpMounter.EndPoint, secretMap["cosEndpoint"])
}

func TestNewMountpointMounter_MountOptsInSecret_Invalid(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":  "test-endpoint",
		"bucketName":   "test-bucket-name",
		"objPath":      "test-obj-path",
		"accessKey":    "test-access-key",
		"secretKey":    "test-secret-key",
		"apiKey":       "test-api-key",
		"mountOptions": "upload_concurrency",
	}
	mounter := NewMountpointMounter(secretMap, mountOptionsMountpoint, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Errorf("NewMountpointMounter() failed to return an instance of MountpointMounter")
	}

	assert.Equal(t, mntpMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, mntpMounter.ObjPath, secretMap["objPath"])
	assert.Equal(t, mntpMounter.EndPoint, secretMap["cosEndpoint"])
}

func Test_MountpointMount_Positive(t *testing.T) {
	mounter := NewMountpointMounter(secretMapMountpoint, mountOptionsMountpoint,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))
	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()
	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Fatal("NewMountpointMounter() did not return a MountpointMounter")
	}

	target := "/tmp/test-mount"

	err := mntpMounter.Mount("source", target)
	assert.NoError(t, err)
}

func Test_MountpointMount_Positive_Empty_ObjPath(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint": "test-endpoint",
		"bucketName":  "test-bucket-name",
		"accessKey":   "test-access-key",
		"secretKey":   "test-secret-key",
		"apiKey":      "test-api-key",
	}
	mounter := NewMountpointMounter(secretMap, mountOptionsMountpoint,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Fatal("NewMountpointMounter() did not return a MountpointMounter")
	}

	target := "/tmp/test-mount"

	err := mntpMounter.Mount("source", target)
	assert.NoError(t, err)
}

func Test_MountpointMount_Error_Creating_Mount_Point(t *testing.T) {
	mounter := NewMountpointMounter(secretMapMountpoint, mountOptionsMountpoint,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))

	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Fatal("NewMountpointMounter() did not return a MountpointMounter")
	}

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return errors.New("error creating mount path")
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	target := "/tmp/test-mount"

	err := mntpMounter.Mount("source", target)
	assert.Error(t, err, "Cannot create directory")
}

func Test_MountpointMount_ErrorMount(t *testing.T) {
	mounter := NewMountpointMounter(secretMapMountpoint, mountOptionsMountpoint,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return errors.New("error mounting volume")
			},
		}))

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Fatal("NewMountpointMounter() did not return a MountpointMounter")
	}

	target := "/tmp/test-mount"

	err := mntpMounter.Mount("source", target)
	assert.Error(t, err, "error mounting volume")
}

func Test_MountpointUnmount_Positive(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint": "test-endpoint",
		"bucketName":  "test-bucket-name",
		"objPath":     "test-obj-path",
		"accessKey":   "test-access-key",
		"secretKey":   "test-secret-key",
		"apiKey":      "test-api-key",
	}
	mounter := NewMountpointMounter(secretMap, []string{"mountOption1", "mountOption2"},
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}))

	mntpMounter, ok := mounter.(*MountpointMounter)
	if !ok {
		t.Fatal("NewMountpointMounter() did not return a MountpointMounter")
	}

	target := "/tmp/test-unmount"

	// Creating a directory to simulate a mounted path
	err := os.MkdirAll(target, os.ModePerm)
	if err != nil {
		t.Fatalf("Test_MountpointUnmount_Positive() failed to create directory: %v", err)
	}

	err = mntpMounter.Unmount(target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("Test_MountpointUnmount_Positive() failed to unmount: %v", err)
	}

	err = os.RemoveAll(target)
	if err != nil {
		t.Errorf("Failed to remove directory: %v", err)
	}
}

func Test_MountpointUnmount_Error(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint": "test-endpoint",
		"bucketName":  "test-bucket-name",
		"objPath":     "test-obj-path",
		"accessKey":   "test-access-key",
		"secretKey":   "test-secret-key",
		"apiKey":      "test-api-key",
	}
	mounter := NewMountpointMounter(secretMap, []string{"mountOption1", "mountOption2"},
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return errors.New("error unmounting volume")
			},
		}))

	mntpMounter := mounter.(*MountpointMounter)

	target := "/tmp/test-unmount"

	// Creating a directory to simulate a mounted path
	err := os.MkdirAll(target, os.ModePerm)
	if err != nil {
		t.Fatalf("TestMountpointUnmount_Error() failed to create directory: %v", err)
	}

	err = mntpMounter.Unmount(target)
	assert.Error(t, err, "error unmounting volume")

	err = os.RemoveAll(target)
	if err != nil {
		t.Errorf("Failed to remove directory: %v", err)
	}
}

func TestUpdateMountpointMountOptions(t *testing.T) {
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
