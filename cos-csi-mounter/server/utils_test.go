package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/stretchr/testify/assert"
)

func TestRedactArgsForLog_MasksPasswdFile(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://s3.example.com","passwd_file":"/var/lib/coscsi-config/secret.txt","allow_other":"true"}`)
	result := redactArgsForLog(raw)

	var m map[string]interface{}
	err := json.Unmarshal(result, &m)
	assert.NoError(t, err)
	assert.Equal(t, "xxxxx", m["passwd_file"])
	assert.Equal(t, "https://s3.example.com", m["url"])
	assert.Equal(t, "true", m["allow_other"])
}

func TestRedactArgsForLog_NoPasswdFile(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://s3.example.com","allow_other":"true"}`)
	result := redactArgsForLog(raw)

	var m map[string]interface{}
	err := json.Unmarshal(result, &m)
	assert.NoError(t, err)
	assert.NotContains(t, m, "passwd_file")
	assert.Equal(t, "https://s3.example.com", m["url"])
}

func TestRedactArgsForLog_EmptyInput(t *testing.T) {
	result := redactArgsForLog(json.RawMessage{})
	assert.Empty(t, result)
}

func TestRedactArgsForLog_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	result := redactArgsForLog(raw)
	assert.Equal(t, []byte(`{invalid}`), []byte(result))
}

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
