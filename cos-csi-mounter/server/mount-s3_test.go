package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/stretchr/testify/assert"
)

var (
	testCredFile = "/var/lib/coscsi-config/test/credentials"
	testCfgFile  = "/var/lib/coscsi-config/test/config"
	testEndpoint = "https://s3.us-south.cloud-object-storage.appdomain.cloud"
	testRegion   = "us-south-standard"
)

// mockS3FileExists replaces FileExists for the duration of the test.
func mockS3FileExists(t *testing.T, exists bool, err error) {
	t.Helper()
	orig := FileExists
	FileExists = func(string) (bool, error) { return exists, err }
	t.Cleanup(func() { FileExists = orig })
}

// --- PopulateArgsSlice tests ---

func TestS3MounterPopulateArgsSlice_BasicArgs(t *testing.T) {
	args := s3MounterArgs{
		AllowOther:         "true",
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		EndpointURL:        testEndpoint,
		Region:             testRegion,
		AllowDelete:        "true",
		AllowOverwrite:     "true",
		ForcePathStyle:     "true",
	}

	result, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Equal(t, testBucket, result[0])
	assert.Equal(t, testTargetPath, result[1])
	assert.Contains(t, result, "--allow-other")
	assert.Contains(t, result, "--allow-delete")
	assert.Contains(t, result, "--allow-overwrite")
	assert.Contains(t, result, "--endpoint-url="+testEndpoint)
	assert.Contains(t, result, "--region="+testRegion)
	assert.Contains(t, result, "--force-path-style")
}

func TestS3MounterPopulateArgsSlice_ReadOnly(t *testing.T) {
	args := s3MounterArgs{
		AllowOther: "true",
		ReadOnly:   "TRUE",
	}

	result, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, result, "--read-only")
	assert.NotContains(t, result, "--allow-delete")
	assert.NotContains(t, result, "--allow-overwrite")
}

// Validate accepts any casing of "true"/"false", so PopulateArgsSlice must too.
func TestS3MounterPopulateArgsSlice_BoolIsCaseInsensitive(t *testing.T) {
	args := s3MounterArgs{AllowOther: "TRUE", ForcePathStyle: " True "}

	result, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, result, "--allow-other")
	assert.Contains(t, result, "--force-path-style")
}

func TestS3MounterPopulateArgsSlice_UIDAndGID(t *testing.T) {
	args := s3MounterArgs{
		UID: "1000",
		GID: "2000",
	}

	result, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, result, "--uid=1000")
	assert.Contains(t, result, "--gid=2000")
}

func TestS3MounterPopulateArgsSlice_LogLevels(t *testing.T) {
	for _, level := range []string{"debug", "debug-crt", "no-log"} {
		args := s3MounterArgs{LogLevel: level}
		result, err := args.PopulateArgsSlice(testBucket, testTargetPath)
		assert.NoError(t, err)
		assert.Contains(t, result, "--"+level)
	}
}

func TestS3MounterPopulateArgsSlice_AddMountParam(t *testing.T) {
	args := s3MounterArgs{
		AddMountParam: "--max-threads=32, --read-part-size=16777216,,--auto-unmount",
	}

	result, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, result, "--max-threads=32")
	assert.Contains(t, result, "--read-part-size=16777216")
	assert.Contains(t, result, "--auto-unmount")
	// the transport key itself must not be emitted as a flag
	for _, v := range result {
		assert.NotContains(t, v, "add-mount-param")
	}
}

func TestS3MounterPopulateArgsSlice_CacheAndLogDir(t *testing.T) {
	args := s3MounterArgs{
		CacheDir:     "/tmp/cache",
		LogDirectory: "/var/log/mount-s3",
	}

	result, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	assert.Contains(t, result, "--cache=/tmp/cache")
	assert.Contains(t, result, "--log-directory=/var/log/mount-s3")
}

// --- Validate tests ---

func TestS3MounterValidate_Success(t *testing.T) {
	mockS3FileExists(t, true, nil)
	args := s3MounterArgs{
		AllowOther:         "true",
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
	}

	err := args.Validate(testTargetPath)
	assert.NoError(t, err)
}

func TestS3MounterValidate_InvalidPath(t *testing.T) {
	args := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
	}

	err := args.Validate("invalid-path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestS3MounterValidate_MissingCredentialsFile(t *testing.T) {
	args := s3MounterArgs{
		AwsConfigFile: testCfgFile,
	}

	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aws-credentials-file is required")
}

func TestS3MounterValidate_MissingConfigFile(t *testing.T) {
	mockS3FileExists(t, true, nil)
	args := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
	}

	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aws-config-file is required")
}

func TestS3MounterValidate_FilesNotFound(t *testing.T) {
	mockS3FileExists(t, false, nil)

	err := s3MounterArgs{AwsCredentialsFile: testCredFile, AwsConfigFile: testCfgFile}.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aws-credentials-file not found")
}

func TestS3MounterValidate_ConfigFileNotFound(t *testing.T) {
	orig := FileExists
	t.Cleanup(func() { FileExists = orig })
	FileExists = func(path string) (bool, error) { return path == testCredFile, nil }

	err := s3MounterArgs{AwsCredentialsFile: testCredFile, AwsConfigFile: testCfgFile}.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aws-config-file not found")
}

func TestS3MounterValidate_FileCheckError(t *testing.T) {
	mockS3FileExists(t, false, errors.New("stat failed"))

	err := s3MounterArgs{AwsCredentialsFile: testCredFile, AwsConfigFile: testCfgFile}.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error checking aws-credentials-file existence")
}

// Uses the real FileExists: paths outside the safe mounter config dir are rejected.
func TestS3MounterValidate_FileOutsideSafeDir(t *testing.T) {
	orig := FileExists
	FileExists = fileExists
	t.Cleanup(func() { FileExists = orig })

	err := s3MounterArgs{AwsCredentialsFile: "/etc/passwd", AwsConfigFile: testCfgFile}.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error checking aws-credentials-file existence")

	err = s3MounterArgs{AwsCredentialsFile: testCredFile, AwsConfigFile: "/etc/passwd"}.Validate(testTargetPath)
	assert.Error(t, err)
}

func TestS3MounterValidate_InvalidAllowOther(t *testing.T) {
	args := s3MounterArgs{
		AllowOther:         "yes",
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
	}

	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert value of allow-other into boolean")
}

func TestS3MounterValidate_InvalidBoolFields(t *testing.T) {
	base := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
	}

	tests := []struct {
		name   string
		apply  func(*s3MounterArgs)
		errMsg string
	}{
		{"invalid read-only", func(a *s3MounterArgs) { a.ReadOnly = "yes" }, "cannot convert value of read-only"},
		{"invalid allow-delete", func(a *s3MounterArgs) { a.AllowDelete = "yes" }, "cannot convert value of allow-delete"},
		{"invalid allow-overwrite", func(a *s3MounterArgs) { a.AllowOverwrite = "yes" }, "cannot convert value of allow-overwrite"},
		{"invalid incremental-upload", func(a *s3MounterArgs) { a.IncrementalUpload = "yes" }, "cannot convert value of incremental-upload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := base
			tt.apply(&args)
			err := args.Validate(testTargetPath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestS3MounterValidate_InvalidUID(t *testing.T) {
	mockS3FileExists(t, true, nil)
	args := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		UID:                "notanint",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uid must be a valid integer")
}

func TestS3MounterValidate_InvalidGID(t *testing.T) {
	mockS3FileExists(t, true, nil)
	args := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		GID:                "notanint",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gid must be a valid integer")
}

func TestS3MounterValidate_InvalidForcePathStyle(t *testing.T) {
	args := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		ForcePathStyle:     "yes",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert value of force-path-style into boolean")
}

func TestS3MounterValidate_IncrementalUploadRejected(t *testing.T) {
	mockS3FileExists(t, true, nil)
	args := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		IncrementalUpload:  "true",
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incremental-upload is not supported")
}

func TestS3MounterValidate_ReadOnlyWithWriteFlagsRejected(t *testing.T) {
	mockS3FileExists(t, true, nil)
	for name, apply := range map[string]func(*s3MounterArgs){
		"allow-delete":    func(a *s3MounterArgs) { a.AllowDelete = "true" },
		"allow-overwrite": func(a *s3MounterArgs) { a.AllowOverwrite = "true" },
	} {
		t.Run(name, func(t *testing.T) {
			args := s3MounterArgs{
				AwsCredentialsFile: testCredFile,
				AwsConfigFile:      testCfgFile,
				ReadOnly:           "true",
			}
			apply(&args)
			err := args.Validate(testTargetPath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "read-only cannot be combined")
		})
	}
}

func TestS3MounterValidate_LogLevel(t *testing.T) {
	mockS3FileExists(t, true, nil)
	base := s3MounterArgs{AwsCredentialsFile: testCredFile, AwsConfigFile: testCfgFile}

	for _, level := range []string{"", "debug", "debug-crt", "no-log"} {
		args := base
		args.LogLevel = level
		assert.NoError(t, args.Validate(testTargetPath), level)
	}

	args := base
	args.LogLevel = "verbose"
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported log-level")
}

func TestS3MounterValidate_ValidUIDAndGID(t *testing.T) {
	mockS3FileExists(t, true, nil)
	args := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		UID:                "1000",
		GID:                "2000",
	}
	err := args.Validate(testTargetPath)
	assert.NoError(t, err)
}
func TestS3MounterValidate_LogDirectory(t *testing.T) {
	mockS3FileExists(t, true, nil)
	// A log directory inside the safe base is accepted and NOT created by Validate
	// (mount-s3 creates it itself).
	t.Run("inside safe base is not created", func(t *testing.T) {
		missing := filepath.Join(s3MounterSafeLogDir, "validate-test-should-not-exist")
		args := s3MounterArgs{
			AwsCredentialsFile: testCredFile,
			AwsConfigFile:      testCfgFile,
			LogDirectory:       missing,
		}
		assert.NoError(t, args.Validate(testTargetPath))
		_, err := os.Stat(missing)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("outside safe base is rejected", func(t *testing.T) {
		args := s3MounterArgs{
			AwsCredentialsFile: testCredFile,
			AwsConfigFile:      testCfgFile,
			LogDirectory:       "/etc/mount-s3",
		}
		err := args.Validate(testTargetPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid log directory")
	})
}

// --- ParseMounterArgs with AMAZONS3MOUNTER tests (N2) ---

func TestParseMounterArgs_AmazonS3_Valid(t *testing.T) {
	mockS3FileExists(t, true, nil)
	argsStruct := s3MounterArgs{
		AllowOther:         "true",
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		EndpointURL:        testEndpoint,
		Region:             testRegion,
		AllowDelete:        "true",
		AllowOverwrite:     "true",
		ForcePathStyle:     "true",
	}
	b, _ := json.Marshal(argsStruct)

	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.AMAZONS3MOUNTER,
		Args:    b,
	}

	args, err := req.ParseMounterArgs()
	assert.NoError(t, err)
	assert.NotNil(t, args)
	assert.Contains(t, args, testBucket)
	assert.Contains(t, args, "--allow-other")
	assert.Contains(t, args, "--endpoint-url="+testEndpoint)
}

func TestParseMounterArgs_AmazonS3_AddMountParam(t *testing.T) {
	mockS3FileExists(t, true, nil)
	argsStruct := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
		AddMountParam:      "--max-threads=32,--dir-mode=0755",
	}
	b, _ := json.Marshal(argsStruct)

	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.AMAZONS3MOUNTER,
		Args:    b,
	}

	args, err := req.ParseMounterArgs()
	assert.NoError(t, err)
	assert.Contains(t, args, "--max-threads=32")
	assert.Contains(t, args, "--dir-mode=0755")
}

func TestParseMounterArgs_AmazonS3_InvalidJSON(t *testing.T) {
	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.AMAZONS3MOUNTER,
		Args:    json.RawMessage(`{"invalid-json"}`),
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
	assert.Contains(t, err.Error(), "invalid mount-s3 args decode error")
}

func TestParseMounterArgs_AmazonS3_UnknownField(t *testing.T) {
	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.AMAZONS3MOUNTER,
		Args:    json.RawMessage(`{"unknown-field": "value"}`),
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
	assert.Contains(t, err.Error(), "invalid mount-s3 args decode error")
}

func TestParseMounterArgs_AmazonS3_ValidationFails(t *testing.T) {
	argsStruct := s3MounterArgs{
		AllowOther: "true",
	}
	b, _ := json.Marshal(argsStruct)

	req := MountRequest{
		Path:    testTargetPath,
		Bucket:  testBucket,
		Mounter: constants.AMAZONS3MOUNTER,
		Args:    b,
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
	assert.Contains(t, err.Error(), "s3Mounter args validation failed")
}

func TestParseMounterArgs_AmazonS3_InvalidTargetPath(t *testing.T) {
	argsStruct := s3MounterArgs{
		AwsCredentialsFile: testCredFile,
		AwsConfigFile:      testCfgFile,
	}
	b, _ := json.Marshal(argsStruct)

	req := MountRequest{
		Path:    "invalid-path",
		Bucket:  testBucket,
		Mounter: constants.AMAZONS3MOUNTER,
		Args:    b,
	}

	args, err := req.ParseMounterArgs()
	assert.Error(t, err)
	assert.Nil(t, args)
	assert.Contains(t, err.Error(), "s3Mounter args validation failed")
}
