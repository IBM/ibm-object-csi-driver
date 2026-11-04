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

func TestRClonePopulateArgsSlice_MultiThreadCutoff(t *testing.T) {
	args := RCloneArgs{
		MultiThreadCutoff: "256M",
	}

	resp, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	expectedVal := []string{"mount", testBucket, testTargetPath, "--multi-thread-cutoff=256M"}
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

// --- AddMountParam tests ---

func TestRClonePopulateArgsSlice_AddMountParam_AppendedSeparately(t *testing.T) {
	args := RCloneArgs{
		AddMountParam: "--transfers=4,--checkers=8",
	}

	resp, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, resp, "--transfers=4")
	assert.Contains(t, resp, "--checkers=8")
	// add-mount-param itself must NOT appear as a --add-mount-param= argument
	for _, v := range resp {
		assert.NotContains(t, v, "--add-mount-param=")
	}
}

func TestRClonePopulateArgsSlice_AddMountParam_BooleanFlag(t *testing.T) {
	args := RCloneArgs{
		AddMountParam: "--no-traverse",
	}

	resp, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, resp, "--no-traverse")
}

func TestRClonePopulateArgsSlice_AddMountParam_EmptyTokensSkipped(t *testing.T) {
	args := RCloneArgs{
		AddMountParam: "--transfers=4,,  ,--checkers=8",
	}

	resp, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, resp, "--transfers=4")
	assert.Contains(t, resp, "--checkers=8")
}

func TestRCloneValidate_AddMountParam_Valid(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := RCloneArgs{
		AddMountParam: "--transfers=4,--checkers=8,--s3-upload-concurrency=16",
	}
	err := args.Validate(testTargetPath)
	assert.NoError(t, err)
}

func TestRCloneValidate_AddMountParam_BooleanFlagNoValue(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := RCloneArgs{
		AddMountParam: "--no-traverse,--use-mmap",
	}
	err := args.Validate(testTargetPath)
	assert.NoError(t, err)
}

func TestRCloneValidate_AddMountParam_InvalidFormat_NoDoubleDash(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := RCloneArgs{
		AddMountParam: "transfers=4",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "add-mount-param: invalid token")
}

func TestRCloneValidate_AddMountParam_InvalidFormat_SingleDash(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := RCloneArgs{
		AddMountParam: "-transfers=4",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "add-mount-param: invalid token")
}

func TestRCloneValidate_AddMountParam_InvalidFormat_UppercaseFlagName(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := RCloneArgs{
		AddMountParam: "--Transfers=4",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "add-mount-param: invalid token")
}

func TestRCloneValidate_AddMountParam_ReservedFlagName(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	// "config" is a known RCloneArgs field — override attempt must be rejected
	args := RCloneArgs{
		AddMountParam: "--config=/etc/passwd",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "add-mount-param: flag \"config\" is already a known parameter")
}

func TestRCloneValidate_AddMountParam_ReservedFlagName_VfsCache(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	// "vfs-cache-mode" is a known field — must be rejected via add-mount-param
	args := RCloneArgs{
		AddMountParam: "--vfs-cache-mode=full",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "add-mount-param: flag \"vfs-cache-mode\" is already a known parameter")
}

func TestValidateAddMountParam_EmptyString(t *testing.T) {
	err := validateAddMountParam("")
	assert.NoError(t, err)
}

func TestValidateAddMountParam_OnlyCommas(t *testing.T) {
	err := validateAddMountParam(",,,")
	assert.NoError(t, err)
}
