package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/stretchr/testify/assert"
)

func TestParse_UnknownMounter(t *testing.T) {
	req := MountRequest{
		Mounter: "unknown",
	}

	mounter := DefaultMounterArgsParser{}
	args, err := mounter.Parse(req)
	assert.Nil(t, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mounter")
}

func TestParseMounterArgs_S3FS_Valid(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.S3FS,
	}
	argsStruct := S3FSArgs{
		URL: testURL,
	}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.NoError(t, err)
	assert.NotNil(t, args)
}

func TestParseMounterArgs_RClone_Valid(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.RClone,
	}
	argsStruct := RCloneArgs{}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.NoError(t, err)
	assert.NotNil(t, args)
}

func TestParseMounterArgs_S3FS_InvalidJSON(t *testing.T) {
	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.S3FS,
		Args:    json.RawMessage(`{"invalid-json"}`),
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
}

func TestParseMounterArgs_RClone_InvalidJSON(t *testing.T) {
	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.RClone,
		Args:    json.RawMessage(`{"invalid-json"}`),
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
}

func TestParseMounterArgs_S3FS_ValidationFails(t *testing.T) {
	req := MountRequest{
		Path:    "invalid-path",
		Bucket:  testBucket,
		Mounter: constants.S3FS,
	}
	argsStruct := S3FSArgs{}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.Nil(t, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "s3fs args validation failed")
}

func TestParseMounterArgs_RClone_ValidationFails(t *testing.T) {
	req := MountRequest{
		Path:    "invalid-path",
		Bucket:  testBucket,
		Mounter: constants.RClone,
	}
	argsStruct := RCloneArgs{}
	b, _ := json.Marshal(argsStruct)
	req.Args = b

	args, err := req.ParseMounterArgs()
	assert.Nil(t, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rclone args validation failed")
}

func TestFileExists_FileDoesNotExist(t *testing.T) {
	path := filepath.Join(safeMounterConfigDir, "nonexistent-file")
	exists, err := fileExists(path)
	assert.False(t, exists)
	assert.NoError(t, err)
}

func TestFileExists_OutsideSafeDirectory(t *testing.T) {
	path := "/tmp/unsafe-file"
	exists, err := fileExists(path)
	assert.False(t, exists)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside the safe directory")
}

func TestFileExists_AbsFails(t *testing.T) {
	originalAbs := absPathResolver
	defer func() { absPathResolver = originalAbs }()

	absPathResolver = func(path string) (string, error) {
		return "", errors.New("Failed to resolve absolute path")
	}

	exists, err := fileExists("invalid-path")
	assert.False(t, exists)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve absolute path")
}
