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
		"cosEndpoint":        "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		"locationConstraint": "us-east",
		"bucketName":         "test-bucket",
		"objectPath":         "test-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"uid":                "1000",
		"gid":                "1000",
		"mountOptions": `log-level=debug
read-only
cache=/tmp/cache`,
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
	assert.Equal(t, "debug", s3Mounter.LogLevel)
	assert.True(t, s3Mounter.ReadOnly)
	assert.Equal(t, "/tmp/cache", s3Mounter.CacheDir)
	assert.Equal(t, "hmac", s3Mounter.AuthType)
	// MountOptions should contain SC options (dir-mode, file-mode)
	assert.Contains(t, s3Mounter.MountOptions, "--dir-mode=0755")
	assert.Contains(t, s3Mounter.MountOptions, "--file-mode=0644")
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

	// Create a mock that implements envMounter interface
	mockUtils := &mockEnvMounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils:  mockUtils,
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
	assert.Contains(t, err.Error(), "failed to create directory")
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
		BucketName:    "test-bucket",
		ObjectPath:    "test-path",
		EndPoint:      "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		LocConstraint: "us-east",
		UID:           "1000",
		GID:           "2000",
		LogLevel:      "debug",
		LogDirectory:  "/var/log/mount-s3",
		CacheDir:      "/tmp/cache",
		ReadOnly:      true,
		MountOptions:  []string{"--dir-mode=0755", "--max-threads=32"},
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
	assert.Contains(t, nodeServerOp, "--log-directory=/var/log/mount-s3")
	assert.Contains(t, nodeServerOp, "--cache=/tmp/cache")
	assert.Contains(t, nodeServerOp, "--read-only")
	assert.Contains(t, nodeServerOp, "--dir-mode=0755")
	assert.Contains(t, nodeServerOp, "--max-threads=32")

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
	assert.Equal(t, "/var/log/mount-s3", workerNodeOp.LogDirectory)
	assert.Equal(t, "/tmp/cache", workerNodeOp.CacheDir)
	assert.Equal(t, "true", workerNodeOp.ReadOnly)
	assert.Contains(t, workerNodeOp.Args, "--dir-mode=0755")
	assert.Contains(t, workerNodeOp.Args, "--max-threads=32")
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
	assert.Contains(t, err.Error(), "failed to create directory")
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
	assert.Contains(t, err.Error(), "failed to create file")
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
	assert.Contains(t, err.Error(), "failed to chmod")
}

// mockEnvMounter implements the envMounter interface for testing
type mockEnvMounter struct {
	mounterUtils.MounterUtils
}

func (m *mockEnvMounter) FuseMountWithEnv(path string, comm string, args []string, envVars []string) error {
	return nil
}


// Test deduplicatePassthrough function
func TestDeduplicatePassthrough(t *testing.T) {
	tests := []struct {
		name        string
		scFlags     []string
		secretFlags []string
		expected    []string
	}{
		{
			name:        "No overlap - all flags kept",
			scFlags:     []string{"--allow-delete", "--dir-mode=0755"},
			secretFlags: []string{"--max-threads=128"},
			expected:    []string{"--allow-delete", "--dir-mode=0755", "--max-threads=128"},
		},
		{
			name:        "Complete overlap - secret wins",
			scFlags:     []string{"--max-threads=32", "--maximum-throughput-gbps=10"},
			secretFlags: []string{"--max-threads=128", "--maximum-throughput-gbps=25"},
			expected:    []string{"--max-threads=128", "--maximum-throughput-gbps=25"},
		},
		{
			name:        "Partial overlap - secret wins on duplicates",
			scFlags:     []string{"--max-threads=32", "--maximum-throughput-gbps=10", "--allow-delete"},
			secretFlags: []string{"--max-threads=128", "--maximum-throughput-gbps=25"},
			expected:    []string{"--allow-delete", "--max-threads=128", "--maximum-throughput-gbps=25"},
		},
		{
			name:        "Empty SC flags",
			scFlags:     []string{},
			secretFlags: []string{"--max-threads=128"},
			expected:    []string{"--max-threads=128"},
		},
		{
			name:        "Empty secret flags",
			scFlags:     []string{"--max-threads=32"},
			secretFlags: []string{},
			expected:    []string{"--max-threads=32"},
		},
		{
			name:        "Both empty",
			scFlags:     []string{},
			secretFlags: []string{},
			expected:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicatePassthrough(tt.scFlags, tt.secretFlags)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

// Test flagKey function
func TestFlagKey(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		expected string
	}{
		{
			name:     "Flag with value",
			flag:     "--max-threads=128",
			expected: "max-threads",
		},
		{
			name:     "Flag without value",
			flag:     "--allow-delete",
			expected: "allow-delete",
		},
		{
			name:     "Flag without double dash",
			flag:     "max-threads=128",
			expected: "max-threads",
		},
		{
			name:     "Complex flag with multiple equals",
			flag:     "--endpoint-url=https://s3.example.com",
			expected: "endpoint-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flagKey(tt.flag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test splitSecretMountOptions function
func TestSplitSecretMountOptions(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []string
	}{
		{
			name: "Basic multiline options",
			raw: `log-level=debug
read-only
cache=/tmp/cache`,
			expected: []string{"log-level=debug", "read-only", "cache=/tmp/cache"},
		},
		{
			name: "Options with trailing commas",
			raw: `log-level=debug,
read-only,
cache=/tmp/cache,`,
			expected: []string{"log-level=debug", "read-only", "cache=/tmp/cache"},
		},
		{
			name: "Options with comments",
			raw: `log-level=debug
# This is a comment
read-only
cache=/tmp/cache`,
			expected: []string{"log-level=debug", "read-only", "cache=/tmp/cache"},
		},
		{
			name: "Options with empty lines",
			raw: `log-level=debug

read-only

cache=/tmp/cache`,
			expected: []string{"log-level=debug", "read-only", "cache=/tmp/cache"},
		},
		{
			name: "Options with whitespace",
			raw: `  log-level=debug  
  read-only  
  cache=/tmp/cache  `,
			expected: []string{"log-level=debug", "read-only", "cache=/tmp/cache"},
		},
		{
			name:     "Empty string",
			raw:      "",
			expected: nil,
		},
		{
			name: "Only comments and empty lines",
			raw: `# Comment 1

# Comment 2`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitSecretMountOptions(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test parseMountpointS3Options function
func TestParseMountpointS3Options(t *testing.T) {
	tests := []struct {
		name              string
		opts              []string
		expectedMounter   *MountpointS3Mounter
		expectedRemaining []string
	}{
		{
			name: "Parse structured options",
			opts: []string{"--uid=1000", "--gid=2000", "--log-level=debug", "--cache=/tmp/cache", "--read-only"},
			expectedMounter: &MountpointS3Mounter{
				UID:      "1000",
				GID:      "2000",
				LogLevel: "debug",
				CacheDir: "/tmp/cache",
				ReadOnly: true,
			},
			expectedRemaining: []string{},
		},
		{
			name: "Parse with passthrough options",
			opts: []string{"--uid=1000", "--max-threads=32", "--dir-mode=0755"},
			expectedMounter: &MountpointS3Mounter{
				UID: "1000",
			},
			expectedRemaining: []string{"--max-threads=32", "--dir-mode=0755"},
		},
		{
			name: "Parse allow-overwrite and incremental-upload",
			opts: []string{"--allow-overwrite", "--incremental-upload"},
			expectedMounter: &MountpointS3Mounter{
				AllowOverwrite:    true,
				IncrementalUpload: true,
			},
			expectedRemaining: []string{},
		},
		{
			name: "Parse log-directory",
			opts: []string{"--log-directory=/var/log/mount-s3"},
			expectedMounter: &MountpointS3Mounter{
				LogDirectory: "/var/log/mount-s3",
			},
			expectedRemaining: []string{},
		},
		{
			name: "Ignore allow-other",
			opts: []string{"--allow-other", "--uid=1000"},
			expectedMounter: &MountpointS3Mounter{
				UID: "1000",
			},
			expectedRemaining: []string{},
		},
		{
			name: "Add double dash to options without it",
			opts: []string{"max-threads=32", "dir-mode=0755"},
			expectedMounter: &MountpointS3Mounter{},
			expectedRemaining: []string{"--max-threads=32", "--dir-mode=0755"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounter := &MountpointS3Mounter{}
			remaining := parseMountpointS3Options(mounter, tt.opts)
			
			assert.Equal(t, tt.expectedMounter.UID, mounter.UID)
			assert.Equal(t, tt.expectedMounter.GID, mounter.GID)
			assert.Equal(t, tt.expectedMounter.LogLevel, mounter.LogLevel)
			assert.Equal(t, tt.expectedMounter.LogDirectory, mounter.LogDirectory)
			assert.Equal(t, tt.expectedMounter.CacheDir, mounter.CacheDir)
			assert.Equal(t, tt.expectedMounter.ReadOnly, mounter.ReadOnly)
			assert.Equal(t, tt.expectedMounter.AllowOverwrite, mounter.AllowOverwrite)
			assert.Equal(t, tt.expectedMounter.IncrementalUpload, mounter.IncrementalUpload)
			assert.ElementsMatch(t, tt.expectedRemaining, remaining)
		})
	}
}

// Test incremental-upload blocking
func TestNewMountpointS3Mounter_IncrementalUploadBlocked(t *testing.T) {
	secretMap := map[string]string{
		"bucketName": "test-bucket",
		"accessKey":  "test-access-key",
		"secretKey":  "test-secret-key",
		"mountOptions": `incremental-upload
allow-overwrite`,
	}

	mounter := NewMountpointS3Mounter(secretMap, nil, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	s3Mounter, ok := mounter.(*MountpointS3Mounter)
	assert.True(t, ok)
	
	// incremental-upload should be blocked (set to false)
	assert.False(t, s3Mounter.IncrementalUpload, "incremental-upload should be blocked due to data loss issues")
	// allow-overwrite should still be set
	assert.True(t, s3Mounter.AllowOverwrite)
}

// Test SC and secret mount options precedence
func TestNewMountpointS3Mounter_MountOptionsPrecedence(t *testing.T) {
	secretMap := map[string]string{
		"bucketName": "test-bucket",
		"accessKey":  "test-access-key",
		"secretKey":  "test-secret-key",
		"mountOptions": `max-threads=128
maximum-throughput-gbps=25`,
	}
	
	scMountOptions := []string{"--max-threads=32", "--maximum-throughput-gbps=10", "--allow-delete"}

	mounter := NewMountpointS3Mounter(secretMap, scMountOptions, mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}))

	s3Mounter, ok := mounter.(*MountpointS3Mounter)
	assert.True(t, ok)
	
	// Secret options should override SC options for same keys
	assert.Contains(t, s3Mounter.MountOptions, "--max-threads=128")
	assert.Contains(t, s3Mounter.MountOptions, "--maximum-throughput-gbps=25")
	assert.NotContains(t, s3Mounter.MountOptions, "--max-threads=32")
	assert.NotContains(t, s3Mounter.MountOptions, "--maximum-throughput-gbps=10")
	
	// SC-only options should be preserved
	assert.Contains(t, s3Mounter.MountOptions, "--allow-delete")
}

// Test read-only priority resolution in Mount
func TestMountpointS3Mount_ReadOnlyPriority(t *testing.T) {
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

	mockUtils := &mockEnvMounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	}

	s3Mounter := &MountpointS3Mounter{
		MounterUtils:      mockUtils,
		BucketName:        "test-bucket",
		AccessKey:         "test-access-key",
		SecretKey:         "test-secret-key",
		ReadOnly:          true,
		AllowOverwrite:    true, // Should be cleared
		IncrementalUpload: true, // Should be cleared
		MountOptions:      []string{"--allow-delete", "--max-threads=32"}, // allow-delete should be removed
	}

	err := s3Mounter.Mount(source, target)
	assert.NoError(t, err)
	
	// Verify read-only priority cleared conflicting options
	assert.False(t, s3Mounter.AllowOverwrite)
	assert.False(t, s3Mounter.IncrementalUpload)
	assert.NotContains(t, s3Mounter.MountOptions, "--allow-delete")
	assert.Contains(t, s3Mounter.MountOptions, "--max-threads=32")
}
