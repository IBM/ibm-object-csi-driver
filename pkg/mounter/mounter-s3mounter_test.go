package mounter

import (
	"errors"
	"os"
	"testing"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

var (
	s3MounterSecretMap = map[string]string{
		"cosEndpoint":            "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		"locationConstraint":     "us-east",
		"bucketName":             "test-bucket",
		"objectPath":             "test-path",
		"accessKey":              "test-access-key",
		"secretKey":              "test-secret-key",
		"uid":                    "1000",
		"gid":                    "1000",
		"logLevel":               "debug",
		"readOnly":               "true",
		"maxThreads":             "32",
		"readPartSize":           "16777216",
		"writePartSize":          "16777216",
		"maximumThroughputGbps":  "20",
		"uploadChecksums":        "crc32c",
		"cacheDir":               "/tmp/cache",
		"maxCacheSize":           "1024",
		"metadataTTL":            "60",
		"negativeMetadataTTL":    "30",
		"logMetrics":             "true",
	}

	s3MounterMountOptions = []string{"--dir-mode=0755", "--file-mode=0644"}
)

func TestNewMountpointS3Mounter_Success(t *testing.T) {
	mounter := NewMountpointS3Mounter(s3MounterSecretMap, s3MounterMountOptions, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	s3Mounter, ok := mounter.(*MountpointS3Mounter)
	assert.True(t, ok)

	assert.Equal(t, s3MounterSecretMap["bucketName"], s3Mounter.BucketName)
	assert.Equal(t, s3MounterSecretMap["objectPath"], s3Mounter.ObjectPath)
	assert.Equal(t, s3MounterSecretMap["cosEndpoint"], s3Mounter.EndPoint)
	assert.Equal(t, s3MounterSecretMap["locationConstraint"], s3Mounter.LocConstraint)
	assert.Equal(t, s3MounterSecretMap["accessKey"], s3Mounter.AccessKey)
	assert.Equal(t, s3MounterSecretMap["secretKey"], s3Mounter.SecretKey)
	assert.Equal(t, s3MounterSecretMap["uid"], s3Mounter.UID)
	assert.Equal(t, s3MounterSecretMap["gid"], s3Mounter.GID)
	assert.Equal(t, s3MounterSecretMap["logLevel"], s3Mounter.LogLevel)
	assert.True(t, s3Mounter.ReadOnly)
	assert.Equal(t, "hmac", s3Mounter.AuthType)
}

func TestNewMountpointS3Mounter_PerformanceTuning(t *testing.T) {
	mounter := NewMountpointS3Mounter(s3MounterSecretMap, s3MounterMountOptions, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	s3Mounter, ok := mounter.(*MountpointS3Mounter)
	assert.True(t, ok)

	assert.Equal(t, s3MounterSecretMap["maxThreads"], s3Mounter.MaxThreads)
	assert.Equal(t, s3MounterSecretMap["readPartSize"], s3Mounter.ReadPartSize)
	assert.Equal(t, s3MounterSecretMap["writePartSize"], s3Mounter.WritePartSize)
	assert.Equal(t, s3MounterSecretMap["maximumThroughputGbps"], s3Mounter.MaxThroughputGbps)
	assert.Equal(t, s3MounterSecretMap["uploadChecksums"], s3Mounter.UploadChecksums)
	assert.Equal(t, s3MounterSecretMap["cacheDir"], s3Mounter.CacheDir)
	assert.Equal(t, s3MounterSecretMap["maxCacheSize"], s3Mounter.MaxCacheSize)
	assert.Equal(t, s3MounterSecretMap["metadataTTL"], s3Mounter.MetadataTTL)
	assert.Equal(t, s3MounterSecretMap["negativeMetadataTTL"], s3Mounter.NegativeMetadataTTL)
	assert.True(t, s3Mounter.LogMetrics)
}

func TestNewMountpointS3Mounter_GidWithoutUid(t *testing.T) {
	secretMap := map[string]string{
		"gid": "2000",
	}

	mounter := NewMountpointS3Mounter(secretMap, nil, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	s3Mounter, ok := mounter.(*MountpointS3Mounter)
	assert.True(t, ok)
	assert.Equal(t, "2000", s3Mounter.UID)
	assert.Equal(t, "2000", s3Mounter.GID)
}

func TestNewMountpointS3Mounter_UidOverridesGid(t *testing.T) {
	secretMap := map[string]string{
		"uid": "1000",
		"gid": "2000",
	}

	mounter := NewMountpointS3Mounter(secretMap, nil, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	s3Mounter, ok := mounter.(*MountpointS3Mounter)
	assert.True(t, ok)
	assert.Equal(t, "1000", s3Mounter.UID)
	assert.Equal(t, "2000", s3Mounter.GID)
}

func TestMountpointS3Mount_NodeServer_Positive(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(path string, perm os.FileMode) error {
		return nil
	}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
		BucketName:    "test-bucket",
		ObjectPath:    "test-path",
		EndPoint:      "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		LocConstraint: "us-east",
		AccessKey:     "test-access-key",
		SecretKey:     "test-secret-key",
		UID:           "1000",
		GID:           "1000",
		LogLevel:      "debug",
		ReadOnly:      true,
		MountOptions:  s3MounterMountOptions,
	}

	err := s3Mounter.Mount(source, target)
	assert.NoError(t, err)
}

func TestMountpointS3Mount_NodeServer_WithEnvMounter(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(path string, perm os.FileMode) error {
		return nil
	}

	// Create a mock that implements envMounter interface
	mockUtils := &mockEnvMounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils:  mockUtils,
		BucketName:    "test-bucket",
		EndPoint:      "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		LocConstraint: "us-east",
		AccessKey:     "test-access-key",
		SecretKey:     "test-secret-key",
		MountOptions:  s3MounterMountOptions,
	}

	err := s3Mounter.Mount(source, target)
	assert.NoError(t, err)
}

func TestMountpointS3Mount_WorkerNode_Positive(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(path string, perm os.FileMode) error {
		return nil
	}
	mounterRequest = func(payload, url string) error {
		return nil
	}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils:  mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
		BucketName:    "test-bucket",
		EndPoint:      "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		LocConstraint: "us-east",
		AccessKey:     "test-access-key",
		SecretKey:     "test-secret-key",
	}

	err := s3Mounter.Mount(source, target)
	assert.NoError(t, err)
}

func TestMountpointS3Mount_CreateDirFails_Negative(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return errors.New("failed to create directory")
	}

	s3Mounter := &MountpointS3Mounter{
		BucketName: "test-bucket",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
	}

	err := s3Mounter.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot create config dir")
}

func TestMountpointS3Mount_CreateCredFileFails_Negative(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return nil, errors.New("failed to create file")
	}

	s3Mounter := &MountpointS3Mounter{
		BucketName: "test-bucket",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
	}

	err := s3Mounter.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot create file")
}

func TestMountpointS3Mount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(path string, perm os.FileMode) error {
		return nil
	}
	mounterRequest = func(payload, url string) error {
		return errors.New("failed to perform http request")
	}

	s3Mounter := &MountpointS3Mounter{
		BucketName: "test-bucket",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
	}

	err := s3Mounter.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to perform http request")
}

func TestMountpointS3Unmount_NodeServer(t *testing.T) {
	mountWorker = false

	removeS3ConfigFile = func(configPath, target string) {}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}),
	}

	err := s3Mounter.Unmount(target)
	assert.NoError(t, err)
}

func TestMountpointS3Unmount_WorkerNode(t *testing.T) {
	mountWorker = true

	removeS3ConfigFile = func(configPath, target string) {}

	mounterRequest = func(payload, url string) error {
		return nil
	}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}),
	}

	err := s3Mounter.Unmount(target)
	assert.NoError(t, err)
}

func TestMountpointS3Unmount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	mounterRequest = func(payload, url string) error {
		return errors.New("failed to create http request")
	}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}),
	}

	err := s3Mounter.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestMountpointS3Unmount_NodeServer_Negative(t *testing.T) {
	mountWorker = false

	removeS3ConfigFile = func(configPath, target string) {}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return errors.New("failed to unmount")
			},
		}),
	}

	err := s3Mounter.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmount")
}

func TestRemoveS3MountConfigFile_PathNotExists(t *testing.T) {
	Stat = func(path string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	removeS3MountConfigFile("/test", target)
}

func TestRemoveS3MountConfigFile_StatRetryThenSuccess(t *testing.T) {
	attempt := 0
	Stat = func(_ string) (os.FileInfo, error) {
		if attempt == 0 {
			attempt++
			return nil, errors.New("stat error")
		}
		return nil, nil
	}
	RemoveAll = func(_ string) error {
		return nil
	}

	removeS3MountConfigFile("/test", target)
}

func TestRemoveS3MountConfigFile_RemoveRetryThenSuccess(t *testing.T) {
	Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}
	attempt := 0
	RemoveAll = func(_ string) error {
		if attempt == 0 {
			attempt++
			return errors.New("remove error")
		}
		return nil
	}

	removeS3MountConfigFile("/test", target)
}

func TestRemoveS3MountConfigFile_Negative(t *testing.T) {
	called := 0
	Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}
	RemoveAll = func(_ string) error {
		called++
		return errors.New("remove failed")
	}

	removeS3MountConfigFile("/test", target)
	assert.Equal(t, maxRetries, called)
}

func TestFormulateMountOptions_AllOptions(t *testing.T) {
	s3Mounter := &MountpointS3Mounter{
		BucketName:          "test-bucket",
		ObjectPath:          "test-path",
		EndPoint:            "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		LocConstraint:       "us-east",
		UID:                 "1000",
		GID:                 "2000",
		LogLevel:            "debug",
		ReadOnly:            true,
		MaxThreads:          "32",
		ReadPartSize:        "16777216",
		WritePartSize:       "16777216",
		MaxThroughputGbps:   "20",
		UploadChecksums:     "crc32c",
		CacheDir:            "/tmp/cache",
		MaxCacheSize:        "1024",
		MetadataTTL:         "60",
		NegativeMetadataTTL: "30",
		LogMetrics:          true,
		MountOptions:        []string{"--dir-mode=0755"},
	}

	nodeServerOp, envVars, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket/test-path", target, "/config/path")

	// Check node server options
	assert.Contains(t, nodeServerOp, "test-bucket/test-path")
	assert.Contains(t, nodeServerOp, target)
	assert.Contains(t, nodeServerOp, "--allow-other")
	assert.Contains(t, nodeServerOp, "--endpoint-url=https://s3.us-east.cloud-object-storage.appdomain.cloud")
	assert.Contains(t, nodeServerOp, "--region=us-east")
	assert.Contains(t, nodeServerOp, "--uid=1000")
	assert.Contains(t, nodeServerOp, "--gid=2000")
	assert.Contains(t, nodeServerOp, "--debug")
	assert.Contains(t, nodeServerOp, "--read-only")
	assert.Contains(t, nodeServerOp, "--max-threads=32")
	assert.Contains(t, nodeServerOp, "--read-part-size=16777216")
	assert.Contains(t, nodeServerOp, "--write-part-size=16777216")
	assert.Contains(t, nodeServerOp, "--maximum-throughput-gbps=20")
	assert.Contains(t, nodeServerOp, "--upload-checksums=crc32c")
	assert.Contains(t, nodeServerOp, "--cache=/tmp/cache")
	assert.Contains(t, nodeServerOp, "--max-cache-size=1024")
	assert.Contains(t, nodeServerOp, "--metadata-ttl=60")
	assert.Contains(t, nodeServerOp, "--negative-metadata-ttl=30")
	assert.Contains(t, nodeServerOp, "--log-metrics")
	assert.Contains(t, nodeServerOp, "--dir-mode=0755")

	// Check env vars
	assert.Len(t, envVars, 2)
	assert.Contains(t, envVars[0], "AWS_SHARED_CREDENTIALS_FILE=")
	assert.Contains(t, envVars[1], "AWS_CONFIG_FILE=")

	// Check worker node options
	assert.Equal(t, "true", workerNodeOp.AllowOther)
	assert.Equal(t, "https://s3.us-east.cloud-object-storage.appdomain.cloud", workerNodeOp.EndpointURL)
	assert.Equal(t, "us-east", workerNodeOp.Region)
	assert.Equal(t, "1000", workerNodeOp.UID)
	assert.Equal(t, "2000", workerNodeOp.GID)
	assert.Equal(t, "debug", workerNodeOp.LogLevel)
	assert.Equal(t, "true", workerNodeOp.ReadOnly)
	assert.Equal(t, "32", workerNodeOp.MaxThreads)
	assert.Equal(t, "16777216", workerNodeOp.ReadPartSize)
	assert.Equal(t, "16777216", workerNodeOp.WritePartSize)
	assert.Equal(t, "20", workerNodeOp.MaxThroughputGbps)
	assert.Equal(t, "crc32c", workerNodeOp.UploadChecksums)
	assert.Equal(t, "/tmp/cache", workerNodeOp.CacheDir)
	assert.Equal(t, "1024", workerNodeOp.MaxCacheSize)
	assert.Equal(t, "60", workerNodeOp.MetadataTTL)
	assert.Equal(t, "30", workerNodeOp.NegativeMetadataTTL)
	assert.True(t, workerNodeOp.LogMetrics)
}

func TestFormulateMountOptions_LogLevelDebugCrt(t *testing.T) {
	s3Mounter := &MountpointS3Mounter{
		LogLevel: "debug-crt",
	}

	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")

	assert.Contains(t, nodeServerOp, "--debug-crt")
	assert.Equal(t, "debug-crt", workerNodeOp.LogLevel)
}

func TestFormulateMountOptions_LogLevelNoLog(t *testing.T) {
	s3Mounter := &MountpointS3Mounter{
		LogLevel: "no-log",
	}

	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")

	assert.Contains(t, nodeServerOp, "--no-log")
	assert.Equal(t, "no-log", workerNodeOp.LogLevel)
}

func TestFormulateMountOptions_InvalidLogLevel(t *testing.T) {
	s3Mounter := &MountpointS3Mounter{
		LogLevel: "invalid-level",
	}

	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")

	// Should not contain any log level flag
	assert.NotContains(t, nodeServerOp, "--debug")
	assert.NotContains(t, nodeServerOp, "--debug-crt")
	assert.NotContains(t, nodeServerOp, "--no-log")
	assert.Empty(t, workerNodeOp.LogLevel)
}

func TestFormulateMountOptions_MinimalConfig(t *testing.T) {
	s3Mounter := &MountpointS3Mounter{
		MountOptions: []string{},
	}

	nodeServerOp, envVars, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")

	// Check basic options are present
	assert.Contains(t, nodeServerOp, "test-bucket")
	assert.Contains(t, nodeServerOp, target)
	assert.Contains(t, nodeServerOp, "--allow-other")

	// Check env vars
	assert.Len(t, envVars, 2)

	// Check worker node options
	assert.Equal(t, "true", workerNodeOp.AllowOther)
}

func TestCreateS3MountConfig_Success(t *testing.T) {
	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(path string, perm os.FileMode) error {
		return nil
	}

	s3Mounter := &MountpointS3Mounter{
		AccessKey:     "test-access-key",
		SecretKey:     "test-secret-key",
		LocConstraint: "us-east",
		EndPoint:      "https://s3.us-east.cloud-object-storage.appdomain.cloud",
	}

	err := createS3MountConfig("/config/path", s3Mounter)
	assert.NoError(t, err)
}

func TestCreateS3MountConfig_MakeDirFails(t *testing.T) {
	MakeDir = func(path string, perm os.FileMode) error {
		return errors.New("failed to create directory")
	}

	s3Mounter := &MountpointS3Mounter{
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	}

	err := createS3MountConfig("/config/path", s3Mounter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot create config dir")
}

func TestCreateS3MountConfig_CreateFileFails(t *testing.T) {
	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return nil, errors.New("failed to create file")
	}

	s3Mounter := &MountpointS3Mounter{
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	}

	err := createS3MountConfig("/config/path", s3Mounter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot write credentials file")
}

func TestCreateS3MountConfig_ChmodFails(t *testing.T) {
	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	CreateFile = func(path string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(path string, perm os.FileMode) error {
		return errors.New("failed to chmod")
	}

	s3Mounter := &MountpointS3Mounter{
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	}

	err := createS3MountConfig("/config/path", s3Mounter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot write credentials file")
}

// mockEnvMounter implements the envMounter interface for testing
type mockEnvMounter struct {
	mounterUtils.MounterUtils
}

func (m *mockEnvMounter) FuseMountWithEnv(path string, comm string, args []string, envVars []string) error {
	return nil
}
