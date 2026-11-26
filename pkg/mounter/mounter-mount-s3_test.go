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
		"mountOptions": `log-level=debug
cache=/tmp/cache`,
	}

	s3MounterMountOptions = []string{"--dir-mode=0755", "--file-mode=0644"}
)

func TestNewMountS3Mounter_Success(t *testing.T) {
	mounter := NewMountS3Mounter(MountS3MounterParams{
		SecretMap:    s3MounterSecretMap,
		MountOptions: s3MounterMountOptions,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
		Gid:          "1000",
		ReadOnly:     true,
	})

	s3Mounter, ok := mounter.(*MountS3Mounter)
	assert.True(t, ok)

	assert.Equal(t, s3MounterSecretMap["bucketName"], s3Mounter.BucketName)
	assert.Equal(t, s3MounterSecretMap["objectPath"], s3Mounter.ObjectPath)
	assert.Equal(t, s3MounterSecretMap["cosEndpoint"], s3Mounter.EndPoint)
	assert.Equal(t, s3MounterSecretMap["locationConstraint"], s3Mounter.LocConstraint)
	assert.Equal(t, s3MounterSecretMap["accessKey"], s3Mounter.AccessKey)
	assert.Equal(t, s3MounterSecretMap["secretKey"], s3Mounter.SecretKey)
	assert.Equal(t, s3MounterSecretMap["uid"], s3Mounter.UID)
	assert.Equal(t, "1000", s3Mounter.GID)
	assert.Equal(t, "debug", s3Mounter.LogLevel)
	assert.True(t, s3Mounter.ReadOnly)
	assert.Equal(t, "/tmp/cache", s3Mounter.CacheDir)
	assert.Equal(t, "hmac", s3Mounter.AuthType)
	// MountOptions should contain SC options (dir-mode, file-mode)
	assert.Contains(t, s3Mounter.MountOptions, "--dir-mode=0755")
	assert.Contains(t, s3Mounter.MountOptions, "--file-mode=0644")
}

func TestNewMountS3Mounter_GidWithoutUid(t *testing.T) {
	secretMap := map[string]string{}

	mounter := NewMountS3Mounter(MountS3MounterParams{
		SecretMap:    secretMap,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
		Gid:          "2000",
	})

	s3Mounter, ok := mounter.(*MountS3Mounter)
	assert.True(t, ok)
	// When gid is set and no uid in secretMap, UID is also set to gid value.
	assert.Equal(t, "2000", s3Mounter.UID)
	assert.Equal(t, "2000", s3Mounter.GID)
}

func TestNewMountS3Mounter_UidOverridesGid(t *testing.T) {
	secretMap := map[string]string{
		"uid": "1000",
	}

	mounter := NewMountS3Mounter(MountS3MounterParams{
		SecretMap:    secretMap,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
		Gid:          "2000",
	})

	s3Mounter, ok := mounter.(*MountS3Mounter)
	assert.True(t, ok)
	// uid from secretMap takes precedence over gid param for the UID field.
	assert.Equal(t, "1000", s3Mounter.UID)
	assert.Equal(t, "2000", s3Mounter.GID)
}

func TestMountS3Mount_NodeServer_Positive(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return os.CreateTemp("", "test")
	}

	mockUtils := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseMountWithEnvFn: func(path, comm string, args, envVars []string) error {
			return nil
		},
	})

	s3Mounter := &MountS3Mounter{
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

func TestMountS3Mount_NodeServer_WithEnvMounter(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return os.CreateTemp("", "test")
	}

	mockUtils := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseMountWithEnvFn: func(path, comm string, args, envVars []string) error {
			return nil
		},
	})

	s3Mounter := &MountS3Mounter{
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

func TestMountS3Mount_WorkerNode_Positive(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	mounterRequest = func(payload, url string) error {
		return nil
	}

	s3Mounter := &MountS3Mounter{
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

func TestMountS3Mount_CreateDirFails_Negative(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return errors.New("failed to create directory")
	}

	s3Mounter := &MountS3Mounter{
		BucketName: "test-bucket",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
	}

	err := s3Mounter.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create directory")
}

func TestMountS3Mount_CreateCredFileFails_Negative(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return nil, errors.New("failed to create file")
	}

	s3Mounter := &MountS3Mounter{
		BucketName: "test-bucket",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
	}

	err := s3Mounter.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot create file")
}

func TestMountS3Mount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	mounterRequest = func(payload, url string) error {
		return errors.New("failed to perform http request")
	}

	s3Mounter := &MountS3Mounter{
		BucketName: "test-bucket",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
	}

	err := s3Mounter.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to perform http request")
}

func TestMountS3Unmount_NodeServer(t *testing.T) {
	mountWorker = false

	removeS3ConfigFile = func(configPath, target string) {}

	s3Mounter := &MountS3Mounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}),
	}

	err := s3Mounter.Unmount(target)
	assert.NoError(t, err)
}

func TestMountS3Unmount_WorkerNode(t *testing.T) {
	mountWorker = true

	removeS3ConfigFile = func(configPath, target string) {}

	mounterRequest = func(payload, url string) error {
		return nil
	}

	s3Mounter := &MountS3Mounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseUnmountFn: func(path string) error {
				return nil
			},
		}),
	}

	err := s3Mounter.Unmount(target)
	assert.NoError(t, err)
}

func TestMountS3Unmount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	mounterRequest = func(payload, url string) error {
		return errors.New("failed to create http request")
	}

	s3Mounter := &MountS3Mounter{
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

func TestMountS3Unmount_NodeServer_Negative(t *testing.T) {
	mountWorker = false

	removeS3ConfigFile = func(configPath, target string) {}

	s3Mounter := &MountS3Mounter{
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

func TestFormulateMountOptions_AllOptions(t *testing.T) {
	s3Mounter := &MountS3Mounter{
		BucketName:    "test-bucket",
		ObjectPath:    "test-path",
		EndPoint:      "https://s3.us-east.cloud-object-storage.appdomain.cloud",
		LocConstraint: "us-east",
		UID:           "1000",
		GID:           "2000",
		LogLevel:      "debug",
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
	assert.Equal(t, "/config/path/credentials", workerNodeOp["aws-credentials-file"])
	assert.Equal(t, "/config/path/config", workerNodeOp["aws-config-file"])
	assert.Equal(t, "true", workerNodeOp["allow-other"])
	assert.Equal(t, "https://s3.us-east.cloud-object-storage.appdomain.cloud", workerNodeOp["endpoint-url"])
	assert.Equal(t, "us-east", workerNodeOp["region"])
	assert.Equal(t, "1000", workerNodeOp["uid"])
	assert.Equal(t, "2000", workerNodeOp["gid"])
	assert.Equal(t, "debug", workerNodeOp["log-level"])
	assert.Equal(t, "/var/log/mount-s3", workerNodeOp["log-directory"])
	assert.Equal(t, "/tmp/cache", workerNodeOp["cache"])
	assert.Equal(t, "true", workerNodeOp["read-only"])
	assert.Equal(t, "0755", workerNodeOp["dir-mode"])
	assert.Equal(t, "32", workerNodeOp["max-threads"])
}

func TestFormulateMountOptions_LogLevelDebugCrt(t *testing.T) {
	s3Mounter := &MountS3Mounter{
		LogLevel: "debug-crt",
	}

	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")

	assert.Contains(t, nodeServerOp, "--debug-crt")
	assert.Equal(t, "debug-crt", workerNodeOp["log-level"])
}

func TestFormulateMountOptions_LogLevelNoLog(t *testing.T) {
	s3Mounter := &MountS3Mounter{
		LogLevel: "no-log",
	}

	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")

	assert.Contains(t, nodeServerOp, "--no-log")
	assert.Equal(t, "no-log", workerNodeOp["log-level"])
}

func TestFormulateMountOptions_InvalidLogLevel(t *testing.T) {
	s3Mounter := &MountS3Mounter{
		LogLevel: "invalid-level",
	}

	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")

	// Should not contain any log level flag
	assert.NotContains(t, nodeServerOp, "--debug")
	assert.NotContains(t, nodeServerOp, "--debug-crt")
	assert.NotContains(t, nodeServerOp, "--no-log")
	assert.Empty(t, workerNodeOp["log-level"])
}

func TestFormulateMountOptions_MinimalConfig(t *testing.T) {
	s3Mounter := &MountS3Mounter{
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
	assert.Equal(t, "true", workerNodeOp["allow-other"])
}

func TestCreateS3MountConfig_Success(t *testing.T) {
	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return os.CreateTemp("", "test")
	}

	s3Mounter := &MountS3Mounter{
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

	s3Mounter := &MountS3Mounter{
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
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return nil, errors.New("failed to create file")
	}

	s3Mounter := &MountS3Mounter{
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	}

	err := createS3MountConfig("/config/path", s3Mounter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create file")
}

// Test updateMountS3Options function
func TestUpdateMountS3Options(t *testing.T) {
	tests := []struct {
		name              string
		scOptions         []string
		secretMap         map[string]string
		expectedMounter   MountS3Mounter
		expectedRemaining []string
	}{
		{
			name:      "Structured options are applied to the mounter",
			scOptions: []string{"--uid=1000", "--gid=2000", "--log-level=debug", "--cache=/tmp/cache"},
			secretMap: map[string]string{},
			expectedMounter: MountS3Mounter{
				UID: "1000", GID: "2000", LogLevel: "debug", CacheDir: "/tmp/cache",
			},
			expectedRemaining: []string{},
		},
		{
			name:              "Passthrough options keep their form",
			scOptions:         []string{"--max-threads=32", "--dir-mode=0755", "--auto-unmount"},
			secretMap:         map[string]string{},
			expectedRemaining: []string{"--max-threads=32", "--dir-mode=0755", "--auto-unmount"},
		},
		{
			name:              "Options without double dash get one",
			scOptions:         []string{"max-threads=32", "dir-mode=0755"},
			secretMap:         map[string]string{},
			expectedRemaining: []string{"--max-threads=32", "--dir-mode=0755"},
		},
		{
			name:              "Secret overrides SC on the same key",
			scOptions:         []string{"--max-threads=32", "--maximum-throughput-gbps=10", "--dir-mode=0755"},
			secretMap:         map[string]string{"mountOptions": "max-threads=128\nmaximum-throughput-gbps=25"},
			expectedRemaining: []string{"--dir-mode=0755", "--max-threads=128", "--maximum-throughput-gbps=25"},
		},
		{
			name:              "Secret key without double dash overrides SC key with it",
			scOptions:         []string{"--max-threads=32"},
			secretMap:         map[string]string{"mountOptions": "max-threads=128"},
			expectedRemaining: []string{"--max-threads=128"},
		},
		{
			name:              "Duplicate keys within one source collapse to the last",
			scOptions:         []string{"--max-threads=32", "--max-threads=64"},
			secretMap:         map[string]string{},
			expectedRemaining: []string{"--max-threads=64"},
		},
		{
			name:              "Value containing equals is kept intact",
			scOptions:         []string{"--prefix=a=b"},
			secretMap:         map[string]string{},
			expectedRemaining: []string{"--prefix=a=b"},
		},
		{
			name:              "Secret options with commas, comments, blanks and whitespace",
			secretMap:         map[string]string{"mountOptions": "  max-threads=32,  \n# comment\n\n--dir-mode=0755\n"},
			expectedRemaining: []string{"--max-threads=32", "--dir-mode=0755"},
		},
		{
			name:      "Structured option without a value is ignored",
			scOptions: []string{"--uid", "--gid", "--log-level", "--cache"},
			secretMap: map[string]string{},

			expectedRemaining: []string{},
		},
		{
			name:              "Empty input",
			secretMap:         map[string]string{},
			expectedRemaining: []string{},
		},
		{
			name:              "Nil secret map",
			scOptions:         []string{"--max-threads=32"},
			secretMap:         nil,
			expectedRemaining: []string{"--max-threads=32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounter := &MountS3Mounter{}
			remaining := updateMountS3Options(mounter, tt.scOptions, tt.secretMap)

			assert.Equal(t, tt.expectedMounter.UID, mounter.UID)
			assert.Equal(t, tt.expectedMounter.GID, mounter.GID)
			assert.Equal(t, tt.expectedMounter.LogLevel, mounter.LogLevel)
			assert.Equal(t, tt.expectedMounter.CacheDir, mounter.CacheDir)
			assert.ElementsMatch(t, tt.expectedRemaining, remaining)
		})
	}
}

// Test that structured options from the secret override the ones from the SC.
func TestUpdateMountS3Options_SecretOverridesStructured(t *testing.T) {
	mounter := &MountS3Mounter{}
	remaining := updateMountS3Options(mounter,
		[]string{"--uid=1", "--log-level=debug"},
		map[string]string{"mountOptions": "uid=2\nlog-level=no-log"})

	assert.Empty(t, remaining)
	assert.Equal(t, "2", mounter.UID)
	assert.Equal(t, "no-log", mounter.LogLevel)
}

// Test that every driver-managed or unsupported flag is dropped, with or
// without the leading "--" and with or without a value, and never reaches
// the passthrough slice or changes the mounter.
func TestUpdateMountS3Options_IgnoredOptions(t *testing.T) {
	for key := range s3MountIgnoredOptions {
		for _, opt := range []string{"--" + key, key, "--" + key + "=value"} {
			t.Run(opt, func(t *testing.T) {
				mounter := &MountS3Mounter{}
				remaining := updateMountS3Options(mounter, []string{opt, "--max-threads=32"}, nil)

				assert.Equal(t, []string{"--max-threads=32"}, remaining)
				assert.Equal(t, &MountS3Mounter{}, mounter)
			})
		}
	}
}

// Test incremental-upload blocking
func TestNewMountS3Mounter_IncrementalUploadBlocked(t *testing.T) {
	secretMap := map[string]string{
		"bucketName": "test-bucket",
		"accessKey":  "test-access-key",
		"secretKey":  "test-secret-key",
		"mountOptions": `incremental-upload
allow-overwrite`,
	}

	mounter := NewMountS3Mounter(MountS3MounterParams{
		SecretMap:    secretMap,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	})

	s3Mounter, ok := mounter.(*MountS3Mounter)
	assert.True(t, ok)

	// incremental-upload should be blocked due to data loss issues.
	assert.NotContains(t, s3Mounter.MountOptions, "--incremental-upload")
	// allow-overwrite is hardcoded, so it must not leak into passthrough options.
	assert.NotContains(t, s3Mounter.MountOptions, "--allow-overwrite")

	// allow-overwrite is emitted by formulateMountOptions since read-only is not set.
	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("b", target, "/config/path")
	assert.Contains(t, nodeServerOp, "--allow-overwrite")
	assert.Equal(t, "true", workerNodeOp["allow-overwrite"])
	assert.NotContains(t, nodeServerOp, "--incremental-upload")
	assert.NotContains(t, workerNodeOp, "incremental-upload")
}

// Test SC and secret mount options precedence
func TestNewMountS3Mounter_MountOptionsPrecedence(t *testing.T) {
	secretMap := map[string]string{
		"bucketName": "test-bucket",
		"accessKey":  "test-access-key",
		"secretKey":  "test-secret-key",
		"mountOptions": `max-threads=128
maximum-throughput-gbps=25`,
	}

	// --allow-delete is now hardcoded — it must not appear in passthrough MountOptions.
	scMountOptions := []string{"--max-threads=32", "--maximum-throughput-gbps=10", "--allow-delete"}

	mounter := NewMountS3Mounter(MountS3MounterParams{
		SecretMap:    secretMap,
		MountOptions: scMountOptions,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	})

	s3Mounter, ok := mounter.(*MountS3Mounter)
	assert.True(t, ok)

	// Secret options should override SC options for same keys.
	assert.Contains(t, s3Mounter.MountOptions, "--max-threads=128")
	assert.Contains(t, s3Mounter.MountOptions, "--maximum-throughput-gbps=25")
	assert.NotContains(t, s3Mounter.MountOptions, "--max-threads=32")
	assert.NotContains(t, s3Mounter.MountOptions, "--maximum-throughput-gbps=10")

	// allow-delete is now hardcoded — it must NOT appear in the passthrough slice.
	assert.NotContains(t, s3Mounter.MountOptions, "--allow-delete")
	// But it is always enabled by formulateMountOptions when not read-only.
	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("b", target, "/config/path")
	assert.Contains(t, nodeServerOp, "--allow-delete")
	assert.Equal(t, "true", workerNodeOp["allow-delete"])
}

// Test read-only priority resolution in Mount
func TestMountS3Mount_ReadOnlyPriority(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	OpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		return os.CreateTemp("", "test")
	}

	mockUtils := mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseMountWithEnvFn: func(path, comm string, args, envVars []string) error {
			return nil
		},
	})

	s3Mounter := &MountS3Mounter{
		MounterUtils: mockUtils,
		BucketName:   "test-bucket",
		AccessKey:    "test-access-key",
		SecretKey:    "test-secret-key",
		ReadOnly:     true,
		MountOptions: []string{"--max-threads=32"},
	}

	err := s3Mounter.Mount(source, target)
	assert.NoError(t, err)

	// Verify read-only priority suppresses all write-enabling flags.
	nodeServerOp, _, workerNodeOp := s3Mounter.formulateMountOptions("test-bucket", target, "/config/path")
	assert.Contains(t, nodeServerOp, "--read-only")
	assert.NotContains(t, nodeServerOp, "--allow-delete")
	assert.NotContains(t, nodeServerOp, "--allow-overwrite")
	assert.Empty(t, workerNodeOp["allow-delete"])
	assert.Empty(t, workerNodeOp["allow-overwrite"])
	// Passthrough options unrelated to write-access are preserved.
	assert.Contains(t, s3Mounter.MountOptions, "--max-threads=32")
}
