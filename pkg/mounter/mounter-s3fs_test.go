// Package mounter
package mounter

import (
	"errors"
	"os"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewS3fsMounter_Success(t *testing.T) {
	// Mock the secretMap and mountOptions
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}

	mountOptions := []string{"opt1=val1", "opt2=val2"}

	mounter, err := NewS3fsMounter(secretMap, mountOptions, utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{}))
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
	// Mock the secretMap and mountOptions
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}

	mountOptions := []string{"opt1=val1", "opt2=val2"}

	mounter, err := NewS3fsMounter(secretMap, mountOptions, utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{}))
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
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{
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

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	mockWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the mock function
	writePassFunc = mockWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("S3fsMounter_Mount() returned an unexpected error: %v", err)
	}
}

func Test_Mount_Positive_(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{
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

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	mockWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the mock function
	writePassFunc = mockWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("S3fsMounter_Mount() returned an unexpected error: %v", err)
	}
}

func Test_Mount_Error_Creating_Mount_Point(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{
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

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return errors.New("error creating mount path")
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.Error(t, err, "Cannot create directory")
}

func Test_Mount_Error_Creating_PWFile(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{
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

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	mockWritePass := func(pwFileName string, pwFileContent string) error {
		return errors.New("error creating PWFile")
	}

	// Replace writePassFunc with the mock function
	writePassFunc = mockWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.Error(t, err, "Cannot create file")
}

func Test_Mount_ErrorMount(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{
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

	mockMkdirAll := func(path string, perm os.FileMode) error {
		return nil
	}

	// Replace mkdirAllFunc with the mock function
	mkdirAllFunc = mockMkdirAll
	defer func() { mkdirAllFunc = os.MkdirAll }()

	mockWritePass := func(pwFileName string, pwFileContent string) error {
		return nil
	}

	// Replace writePassFunc with the mock function
	writePassFunc = mockWritePass
	defer func() { writePassFunc = writePass }()

	target := "/tmp/test-mount"

	err = s3fsMounter.Mount("source", target)
	assert.Error(t, err, "error mounting volume")
}

func Test_Unmount_Positive(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{
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
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objPath":            "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
	}
	mounter, err := NewS3fsMounter(secretMap, []string{"mountOption1", "mountOption2"},
		utils.NewMockStatsUtilsImpl(utils.MockStatsUtilsFuncStruct{
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
