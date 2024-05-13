// Package mounter
package mounter

import (
	"errors"
	"os"
	"testing"

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

// Fake the secretMap and mountOptions
var secretMap = map[string]string{
	"cosEndpoint":        "test-endpoint",
	"locationConstraint": "test-loc-constraint",
	"bucketName":         "test-bucket-name",
	"objPath":            "test-obj-path",
	"accessKey":          "test-access-key",
	"secretKey":          "test-secret-key",
	"apiKey":             "test-api-key",
	"kpRootKeyCRN":       "test-kp-root-key-crn",
}

var mountOptions = []string{"opt1=val1", "opt2=val2"}

func TestNewS3fsMounter_Success(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, mountOptions, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))
	if err != nil {
		t.Errorf("NewS3fsMounter failed: %v", err)
	}

	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Errorf("NewS3fsMounter() failed to return an instance of s3fsMounter")
	}

	if s3fsMounter.BucketName != secretMap["bucketName"] {
		t.Errorf("Expected bucketName: %s, got: %s", secretMap["bucketName"], s3fsMounter.BucketName)
	}
	if s3fsMounter.ObjPath != secretMap["objPath"] {
		t.Errorf("Expected objPath: %s, got %s ", secretMap["objPath"], s3fsMounter.ObjPath)
	}
	if s3fsMounter.EndPoint != secretMap["cosEndpoint"] {
		t.Errorf("Expected endPoint: %s, got %s ", secretMap["cosEndpoint"], s3fsMounter.EndPoint)
	}
	if s3fsMounter.LocConstraint != secretMap["locationConstraint"] {
		t.Errorf("Expected locationConstraint: %s, got %s ", secretMap["locationConstraint"], s3fsMounter.LocConstraint)
	}
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
	}

	mounter, err := NewS3fsMounter(secretMap, mountOptions, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))
	if err != nil {
		t.Errorf("NewS3fsMounter failed: %v", err)
	}

	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Errorf("NewS3fsMounter() failed to return an instance of s3fsMounter")
	}

	if s3fsMounter.BucketName != secretMap["bucketName"] {
		t.Errorf("Expected bucketName: %s, got: %s", secretMap["bucketName"], s3fsMounter.BucketName)
	}
	if s3fsMounter.ObjPath != secretMap["objPath"] {
		t.Errorf("Expected objPath: %s, got %s ", secretMap["objPath"], s3fsMounter.ObjPath)
	}
	if s3fsMounter.EndPoint != secretMap["cosEndpoint"] {
		t.Errorf("Expected endPoint: %s, got %s ", secretMap["cosEndpoint"], s3fsMounter.EndPoint)
	}
	if s3fsMounter.LocConstraint != secretMap["locationConstraint"] {
		t.Errorf("Expected locationConstraint: %s, got %s ", secretMap["locationConstraint"], s3fsMounter.LocConstraint)
	}
}

func Test_Mount_Positive(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))
	if err != nil {
		t.Fatalf("NewS3fsMounter() returned an unexpected error: %v", err)
	}
	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the Fake function
	writePassFunc = FakeWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("S3fsMounter_Mount() returned an unexpected error: %v", err)
	}
}

func Test_Mount_Positive_Hmac(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))
	if err != nil {
		t.Fatalf("NewS3fsMounter() returned an unexpected error: %v", err)
	}
	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the Fake function
	writePassFunc = FakeWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("S3fsMounter_Mount() returned an unexpected error: %v", err)
	}
}

func Test_Mount_Positive_Empty_ObjPath(t *testing.T) {
	secretMap = map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))
	if err != nil {
		t.Fatalf("NewS3fsMounter() returned an unexpected error: %v", err)
	}
	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the Fake function
	writePassFunc = FakeWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("S3fsMounter_Mount() returned an unexpected error: %v", err)
	}
}

func Test_Mount_Positive_SingleMountOptions(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))
	if err != nil {
		t.Fatalf("NewS3fsMounter() returned an unexpected error: %v", err)
	}
	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the Fake function
	writePassFunc = FakeWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("S3fsMounter_Mount() returned an unexpected error: %v", err)
	}
}

func Test_Mount_Error_Creating_Mount_Point(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))
	if err != nil {
		t.Fatalf("NewS3fsMounter() returned an unexpected error: %v", err)
	}
	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return errors.New("error creating mount path")
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.Error(t, err, "Cannot create directory")
}

func Test_Mount_Error_Creating_PWFile(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return nil
			},
		}))
	if err != nil {
		t.Fatalf("NewS3fsMounter() returned an unexpected error: %v", err)
	}
	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeWritePass := func(pwFileName string, pwFileContent string) error {
		return errors.New("error creating PWFile")
	}

	// Replace writePassFunc with the Fake function
	writePassFunc = FakeWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.Error(t, err, "Cannot create file")
}

func Test_Mount_ErrorMount(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path string, comm string, args []string) error {
				return errors.New("error mounting volume")
			},
		}))
	if err != nil {
		t.Fatalf("NewS3fsMounter() returned an unexpected error: %v", err)
	}
	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	FakeMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the Fake function
	mkdirAllFunc = FakeMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	FakeWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the Fake function
	writePassFunc = FakeWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.Error(t, err, "error mounting volume")
}

func Test_Unmount_Positive(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}))
	if err != nil {
		t.Fatalf("Failed to create S3fsMounter: %v", err)
	}

	s3fsMounter, ok := mounter.(*S3fsMounter)
	if !ok {
		t.Fatal("NewS3fsMounter() did not return a s3fsMounter")
	}

	target := "/tmp/test-unmount"

	// Creating a directory to simulate a mounted path
	err = os.MkdirAll(target, os.ModePerm)
	if err != nil {
		t.Fatalf("TestS3fsMounter_Unmount() failed to create directory: %v", err)
	}

	err = s3fsMounter.Unmount(target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("TestS3fsMounter_Unmount() failed to unmount: %v", err)
	}

	err = os.RemoveAll(target)
	if err != nil {
		t.Errorf("Failed to remove directory: %v", err)
	}
}

func Test_Unmount_Error(t *testing.T) {
	mounter, err := NewS3fsMounter(secretMap, mountOptions,
		mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return errors.New("error unmounting volume")
			},
		}))
	if err != nil {
		t.Fatalf("Failed to create S3fsMounter: %v", err)
	}

	s3fsMounter := mounter.(*S3fsMounter)

	target := "/tmp/test-unmount"

	// Creating a directory to simulate a mounted path
	err = os.MkdirAll(target, os.ModePerm)
	if err != nil {
		t.Fatalf("TestS3fsMounter_Unmount() failed to create directory: %v", err)
	}

	err = s3fsMounter.Unmount(target)
	assert.Error(t, err, "error unmounting volume")

	err = os.RemoveAll(target)
	if err != nil {
		t.Errorf("Failed to remove directory: %v", err)
	}
}

func TestUpdateS3FSMountOptions(t *testing.T) {
	defaultMountOp := []string{"option1=value1", "option2=value2"}
	secretMap := map[string]string{
		"tmpdir":       "/tmp",
		"use_cache":    "true",
		"gid":          "1001",
		"mountOptions": "additional_option=value3",
	}

	updatedOptions, err := updateS3FSMountOptions(defaultMountOp, secretMap)

	assert.NoError(t, err)
	assert.ElementsMatch(t, updatedOptions, []string{
		"option1=value1",
		"option2=value2",
		"tmpdir=/tmp",
		"use_cache=true",
		"gid=1001",
		"uid=1001",
		"additional_option=value3",
	})
}

func TestUpdateS3FSMountOptions_SecretMapUID(t *testing.T) {
	defaultMountOp := []string{"option1=value1", "option2=value2"}
	secretMap := map[string]string{
		"tmpdir":       "/tmp",
		"use_cache":    "true",
		"gid":          "1001",
		"uid":          "1001",
		"mountOptions": "additional_option=value3",
	}

	updatedOptions, err := updateS3FSMountOptions(defaultMountOp, secretMap)

	assert.NoError(t, err)
	assert.ElementsMatch(t, updatedOptions, []string{
		"option1=value1",
		"option2=value2",
		"tmpdir=/tmp",
		"use_cache=true",
		"gid=1001",
		"uid=1001",
		"additional_option=value3",
	})
}

func TestUpdateS3FSMountOptions_SingleMountOptions(t *testing.T) {
	defaultMountOp := []string{"option1=value1", "option2=value2"}
	secretMap := map[string]string{
		"tmpdir":       "/tmp",
		"use_cache":    "true",
		"gid":          "1001",
		"mountOptions": "value3",
	}

	updatedOptions, err := updateS3FSMountOptions(defaultMountOp, secretMap)

	assert.NoError(t, err)
	assert.ElementsMatch(t, updatedOptions, []string{
		"option1=value1",
		"option2=value2",
		"tmpdir=/tmp",
		"use_cache=true",
		"gid=1001",
		"uid=1001",
		"value3",
	})
}

func TestUpdateS3FSMountOptions_Empty_Mount_Options(t *testing.T) {
	defaultMountOp := []string{"option1=value1", "option2=value2"}
	secretMap := map[string]string{
		"tmpdir":       "/tmp",
		"use_cache":    "true",
		"gid":          "1001",
		"mountOptions": "",
	}

	updatedOptions, err := updateS3FSMountOptions(defaultMountOp, secretMap)

	assert.NoError(t, err)
	assert.ElementsMatch(t, updatedOptions, []string{
		"option1=value1",
		"option2=value2",
		"tmpdir=/tmp",
		"use_cache=true",
		"gid=1001",
		"uid=1001",
	})
}

func TestUpdateS3FSMountOptions_Empty_Default_Mount_Options(t *testing.T) {
	defaultMountOp := []string{}
	secretMap := map[string]string{
		"tmpdir":       "/tmp",
		"use_cache":    "true",
		"gid":          "1001",
		"mountOptions": "additional_option=value3",
	}

	updatedOptions, err := updateS3FSMountOptions(defaultMountOp, secretMap)

	assert.NoError(t, err)
	assert.ElementsMatch(t, updatedOptions, []string{
		"tmpdir=/tmp",
		"use_cache=true",
		"gid=1001",
		"uid=1001",
		"additional_option=value3",
	})
}

func TestUpdateS3FSMountOptions_Invalid_Mount_Options(t *testing.T) {
	defaultMountOp := []string{"option1=value1", "option2=value2"}
	secretMap := map[string]string{
		"tmpdir":       "/tmp",
		"use_cache":    "true",
		"gid":          "1001",
		"mountOptions": "additional=option=value3",
	}

	updatedOptions, err := updateS3FSMountOptions(defaultMountOp, secretMap)

	assert.NoError(t, err)
	assert.ElementsMatch(t, updatedOptions, []string{
		"option1=value1",
		"option2=value2",
		"tmpdir=/tmp",
		"use_cache=true",
		"gid=1001",
		"uid=1001",
	})
}
