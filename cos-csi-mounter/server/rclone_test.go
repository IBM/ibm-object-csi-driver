package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRClonePopulateArgsSlice_Success(t *testing.T) {
	args := RCloneArgs{
		AllowOther: "true",
	}

	resp, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	expectedVal := []string{"mount", testBucket, testTargetPath, "--allow-other=true"}
	assert.Equal(t, expectedVal, resp)
}

func TestRCloneValidate_Success(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := RCloneArgs{}
	err := args.Validate(testTargetPath)
	assert.NoError(t, err)
}

func TestRCloneValidate_PathValidatorFailed(t *testing.T) {
	args := RCloneArgs{}
	err := args.Validate("invalid-path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestRCloneValidate_InvalidS3FSParamValues(t *testing.T) {
	fields := []string{
		"AllowOther",
		"AllowRoot",
		"AsyncRead",
		"Daemon",
		"DirectIO",
		"NoModificationTime",
		"ReadOnly",
		"VfsRefresh",
		"WriteBackCache",
	}
	for _, f := range fields {
		args := RCloneArgs{}

		val := reflect.ValueOf(&args).Elem().FieldByName(f)
		val.SetString("invalid-value")
		err := args.Validate(testTargetPath)
		assert.Error(t, err)
	}
}

func TestRCloneValidate_FailedToCheckPasswordFile(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return false, errors.New("error")
	}
	args := RCloneArgs{}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error checking rclone config file existence")
}

func TestRCloneValidate_PasswordFileNotFound(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return false, nil
	}

	args := RCloneArgs{}

	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rclone config file not found")
}
