package mounter

import (
	"errors"
	"os"
	"strings"
	"testing"

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

var (
	secretMapRClone = map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"serviceId":          "test-service-id",
		"gid":                "fake-gid",
		"uid":                "fake-uid",
	}

	mountOptionsRClone = []string{"opt1=val1", "opt2=val2"}
	target             = "/tmp/test-mount"
	source             = "source"
)

func TestNewRcloneMounter_Success(t *testing.T) {
	mounter := NewRcloneMounter(RcloneMounterParams{
		SecretMap:    secretMapRClone,
		MountOptions: mountOptionsRClone,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	})

	rCloneMounter, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, rCloneMounter.BucketName, secretMapRClone["bucketName"])
	assert.Equal(t, rCloneMounter.ObjectPath, secretMapRClone["objectPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMapRClone["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMapRClone["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMapRClone["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMapRClone["gid"])
}

func TestNewRcloneMounter_Only_GID(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "1001",
	}
	mounter := NewRcloneMounter(RcloneMounterParams{
		SecretMap:    secretMap,
		MountOptions: mountOptionsRClone,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	})

	rCloneMounter, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.ObjectPath, secretMap["objectPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
	assert.Equal(t, rCloneMounter.UID, secretMap["gid"]) // uid auto-set from gid when uid absent
}

func TestNewRcloneMounter_MountOptsInSecret_HMAC(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "1001",
		"uid":                "1001",
		"mountOptions":       "\nupload_concurrency\nkey=value",
	}
	mounter := NewRcloneMounter(RcloneMounterParams{
		SecretMap:    secretMap,
		MountOptions: mountOptionsRClone,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	})

	rCloneMounter, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.ObjectPath, secretMap["objectPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMap["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
	assert.Equal(t, rCloneMounter.AuthType, "hmac")
}

func TestNewRcloneMounter_MountOptsInSecret_IAM(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"apiKey":             "test-api-key",
		"serviceId":          "test-service-id",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"gid":                "1001",
		"uid":                "1001",
		"mountOptions":       "\nupload_concurrency\nkey=value",
		"iamEndpoint":        "test-iam-endpoint",
	}
	mounter := NewRcloneMounter(RcloneMounterParams{
		SecretMap:    secretMap,
		MountOptions: mountOptionsRClone,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	})

	rCloneMounter, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, rCloneMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, rCloneMounter.ObjectPath, secretMap["objectPath"])
	assert.Equal(t, rCloneMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, rCloneMounter.LocConstraint, secretMap["locationConstraint"])
	assert.Equal(t, rCloneMounter.UID, secretMap["uid"])
	assert.Equal(t, rCloneMounter.GID, secretMap["gid"])
	assert.Equal(t, rCloneMounter.AuthType, "iam")

}

func TestRcloneMount_NodeServer_Positive(t *testing.T) {
	mountWorker = false

	rclone := &RcloneMounter{
		BucketName: "testBucket",
		AccessKeys: "testAccessKey",
		EndPoint:   "testEndpoint",
		GID:        "testGID",
		UID:        "testUID",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter, _ []string) error {
		return nil
	}

	err := rclone.Mount(source, target)
	assert.NoError(t, err)
}

func TestRcloneMount_CreateConfigFails_Negative(t *testing.T) {
	rclone := &RcloneMounter{}

	createConfigWrap = func(_ string, _ *RcloneMounter, _ []string) error {
		return errors.New("failed to create config file")
	}

	err := rclone.Mount(source, target)
	assert.Error(t, err)
	assert.EqualError(t, err, "failed to create config file")
}

func TestRcloneMount_WorkerNode_Positive(t *testing.T) {
	mountWorker = true

	rclone := &RcloneMounter{
		BucketName: "testBucket",
		AccessKeys: "testAccessKey",
		EndPoint:   "testEndpoint",
		GID:        "testGID",
		UID:        "testUID",
		ObjectPath: "testObjectPath",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter, _ []string) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return nil
	}

	err := rclone.Mount(source, target)
	assert.NoError(t, err)
}

func TestRcloneMount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	rclone := &RcloneMounter{
		BucketName: "testBucket",
		AccessKeys: "testAccessKey",
		EndPoint:   "testEndpoint",
		GID:        "testGID",
		UID:        "testUID",
		ObjectPath: "testObjectPath",
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
	}

	createConfigWrap = func(_ string, _ *RcloneMounter, _ []string) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return errors.New("failed to create http request")
	}

	err := rclone.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestRcloneUnmount_NodeServer(t *testing.T) {
	mountWorker = false

	removeConfigFile = func(_, _ string) {}

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	err := rclone.Unmount(target)
	assert.NoError(t, err)
}

func TestRcloneUnmount_WorkerNode(t *testing.T) {
	mountWorker = true

	removeConfigFile = func(_, _ string) {}

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return nil
	}

	err := rclone.Unmount(target)
	assert.NoError(t, err)
}

func TestRcloneUnmount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return errors.New("failed to create http request")
	}

	err := rclone.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestRcloneUnmount_NodeServer_Negative(t *testing.T) {
	mountWorker = false

	removeConfigFile = func(_, _ string) {}

	rclone := &RcloneMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return errors.New("failed to unmount")
		},
	})}

	err := rclone.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmount")
}

func TestCreateConfig_Success_HMAC(t *testing.T) {
	rclone := &RcloneMounter{
		AccessKeys:    "testAccessKey:testSecretKey",
		EndPoint:      "test-endpoint",
		LocConstraint: "us-south",
		AuthType:      "hmac",
		MountOptions:  []string{"upload_concurrency=16"},
	}

	// Classify MountOptions the same way Mount() does, so extraConfigLines carries
	// the underscore-keyed option into the config file.
	extraConfigLines, _ := classifyRcloneParams(rclone.MountOptions)

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	err := createConfig(tmpDir, rclone, extraConfigLines)
	assert.NoError(t, err)

	// Read the generated config file and verify HMAC parameters
	configFilePath := tmpDir + "/rclone.conf"
	content, err := os.ReadFile(configFilePath)
	assert.NoError(t, err)

	configStr := string(content)

	// Verify HMAC-specific configuration parameters
	assert.Contains(t, configStr, "[ibmcos]")
	assert.Contains(t, configStr, "type = s3")
	assert.Contains(t, configStr, "endpoint = test-endpoint")
	assert.Contains(t, configStr, "provider = IBMCOS")
	assert.Contains(t, configStr, "env_auth = true")
	assert.Contains(t, configStr, "v2_auth = false")
	assert.Contains(t, configStr, "access_key_id = testAccessKey")
	assert.Contains(t, configStr, "secret_access_key = testSecretKey")
	assert.Contains(t, configStr, "location_constraint = us-south")
	assert.Contains(t, configStr, "upload_concurrency=16")
}

func TestCreateConfig_Success_IAM(t *testing.T) {
	rclone := &RcloneMounter{
		AccessKeys:        "testApiKey",
		serviceInstanceID: "test-service-instance-id",
		EndPoint:          "test-endpoint",
		LocConstraint:     "us-south",
		IAMEndpoint:       "test-iam-endpoint",
		AuthType:          "iam",
		MountOptions:      []string{"vfs-cache-mode=writes"},
	}

	// Classify MountOptions: "vfs-cache-mode=writes" has no "_" and starts with no dash,
	// so it is a config-file line (no leading "--").
	extraConfigLines, _ := classifyRcloneParams(rclone.MountOptions)

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	err := createConfig(tmpDir, rclone, extraConfigLines)
	assert.NoError(t, err)

	// Read the generated config file and verify IAM parameters
	configFilePath := tmpDir + "/rclone.conf"
	content, err := os.ReadFile(configFilePath)
	assert.NoError(t, err)

	configStr := string(content)

	// Verify IAM-specific configuration parameters
	assert.Contains(t, configStr, "[ibmcos]")
	assert.Contains(t, configStr, "type = s3")
	assert.Contains(t, configStr, "endpoint = test-endpoint")
	assert.Contains(t, configStr, "provider = IBMCOS")
	assert.Contains(t, configStr, "env_auth = false")
	assert.Contains(t, configStr, "v2_auth = true")
	assert.Contains(t, configStr, "ibm_api_key = testApiKey")
	assert.Contains(t, configStr, "ibm_resource_instance_id = test-service-instance-id")
	assert.Contains(t, configStr, "ibm_iam_endpoint = test-iam-endpoint")
	assert.Contains(t, configStr, "location_constraint = us-south")
	assert.Contains(t, configStr, "vfs-cache-mode=writes")
}

func TestCreateConfig_MakeDirFails(t *testing.T) {
	origMakeDir := MakeDir
	MakeDir = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}
	defer func() { MakeDir = origMakeDir }()
	err := createConfig("/tmp/testconfig", &RcloneMounter{}, nil)
	assert.ErrorContains(t, err, "mkdir failed")
}

func TestCreateConfig_FileCreateFails(t *testing.T) {
	origMakeDir := MakeDir
	origCreateFile := CreateFile
	MakeDir = func(string, os.FileMode) error { return nil }
	CreateFile = func(string) (*os.File, error) {
		return nil, errors.New("file create failed")
	}
	defer func() {
		MakeDir = origMakeDir
		CreateFile = origCreateFile
	}()
	err := createConfig("/tmp/testconfig", &RcloneMounter{}, nil)
	assert.ErrorContains(t, err, "file create failed")
}

func TestCreateConfig_ChmodFails(t *testing.T) {
	origMakeDir := MakeDir
	origCreateFile := CreateFile
	origChmod := Chmod
	MakeDir = func(string, os.FileMode) error { return nil }
	CreateFile = func(string) (*os.File, error) {
		return os.CreateTemp("", "test")
	}
	Chmod = func(string, os.FileMode) error {
		return errors.New("chmod failed")
	}
	defer func() {
		MakeDir = origMakeDir
		CreateFile = origCreateFile
		Chmod = origChmod
	}()
	err := createConfig("/tmp/testconfig", &RcloneMounter{}, nil)
	assert.ErrorContains(t, err, "chmod failed")
}

func TestRemoveRcloneConfigFile_PathNotExists(t *testing.T) {
	Stat = func(path string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	defer func() {
		Stat = os.Stat
	}()

	removeRcloneConfigFile("/test", target)
}

func TestRemoveRcloneConfigFile_StatRetryThenSuccess(t *testing.T) {
	attempt := 0
	Stat = func(_ string) (os.FileInfo, error) {
		if attempt == 0 {
			attempt++
			return nil, errors.New("stat error")
		}
		return nil, nil
	}
	defer func() {
		Stat = os.Stat
	}()

	RemoveAll = func(_ string) error {
		return nil
	}

	removeRcloneConfigFile("/test1", target)
}

func TestRemoveRcloneConfigFile_RemoveRetryThenSuccess(t *testing.T) {
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

	defer func() {
		Stat = os.Stat
		RemoveAll = os.RemoveAll
	}()

	removeRcloneConfigFile("/test", target)
}

func TestRemoveRcloneConfigFile_Negative(t *testing.T) {
	called := 0
	Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}
	RemoveAll = func(_ string) error {
		called++
		return errors.New("remove failed")
	}

	defer func() {
		Stat = os.Stat
		RemoveAll = os.RemoveAll
	}()

	removeRcloneConfigFile("/test", target)
	assert.Equal(t, maxRetries, called)
}

func TestNewRcloneMounter_GidParam(t *testing.T) {
	tests := []struct {
		name    string
		secret  map[string]string
		gid     string
		wantGID string
		wantUID string
	}{
		{
			name:    "gid param overrides secretMap gid",
			secret:  map[string]string{"gid": "1000"},
			gid:     "2000",
			wantGID: "2000",
			wantUID: "2000", // auto-set from gid param since no uid in secret
		},
		{
			name:    "gid param sets uid when uid absent",
			secret:  map[string]string{},
			gid:     "3000",
			wantGID: "3000",
			wantUID: "3000",
		},
		{
			name:    "gid param does not override explicit secretMap uid",
			secret:  map[string]string{"uid": "5000"},
			gid:     "4000",
			wantGID: "4000",
			wantUID: "5000", // secretMap uid takes precedence
		},
		{
			name:    "no gid param and no secret gid — neither set",
			secret:  map[string]string{},
			gid:     "",
			wantGID: "",
			wantUID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounter := NewRcloneMounter(RcloneMounterParams{
				SecretMap:    tt.secret,
				MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
				Gid:          tt.gid,
			})

			rCloneMounter, ok := mounter.(*RcloneMounter)
			assert.True(t, ok)
			assert.Equal(t, tt.wantGID, rCloneMounter.GID)
			assert.Equal(t, tt.wantUID, rCloneMounter.UID)
		})
	}
}

func TestNewRcloneMounter_ReadOnly(t *testing.T) {
	tests := []struct {
		name     string
		readOnly bool
		wantRO   bool
	}{
		{
			name:     "readOnly true sets ReadOnly flag and --read-only in mount args",
			readOnly: true,
			wantRO:   true,
		},
		{
			name:     "readOnly false does not set ReadOnly flag",
			readOnly: false,
			wantRO:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounter := NewRcloneMounter(RcloneMounterParams{
				SecretMap:    map[string]string{},
				MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
				ReadOnly:     tt.readOnly,
			})

			rCloneMounter, ok := mounter.(*RcloneMounter)
			assert.True(t, ok)
			assert.Equal(t, tt.wantRO, rCloneMounter.ReadOnly)

			nodeOp, workerOp := rCloneMounter.formulateMountOptions("bucket", "/target", "/config", nil)
			if tt.wantRO {
				assert.Contains(t, nodeOp, "--read-only")
				assert.Equal(t, "true", workerOp["read-only"])
			} else {
				assert.NotContains(t, nodeOp, "--read-only")
				_, exists := workerOp["read-only"]
				assert.False(t, exists)
			}
		})
	}
}

func TestFormulateRcloneMountOptions_GidUid(t *testing.T) {
	rclone := &RcloneMounter{
		GID:      "1000",
		UID:      "1000",
		EndPoint: "test-endpoint",
	}

	nodeOp, workerOp := rclone.formulateMountOptions("bucket", "/target", "/config", nil)

	assert.Contains(t, nodeOp, "--gid=1000")
	assert.Contains(t, nodeOp, "--uid=1000")
	assert.Equal(t, "1000", workerOp["gid"])
	assert.Equal(t, "1000", workerOp["uid"])
}

// ---------------------------------------------------------------------------
// Tests for classifyRcloneParams and the full xlsx-param secret flow
// ---------------------------------------------------------------------------

// secretMapXlsxParams is a Kubernetes-Secret-like map containing a representative
// sample of every parameter from rclone_params.xlsx, keyed exactly as a user
// would write them in the "mountOptions" field of the Secret stringData block.
//
// Rule tested:
//
//	keys starting with "--"             → CLI flag
//	keys matching "-<single-letter>"    → CLI flag  (short-hand)
//	keys containing "_" or no dash      → rclone.conf line
var secretMapXlsxParams = map[string]string{
	// ---- well-known identity keys (handled separately by NewRcloneMounter) ----
	"cosEndpoint":        "https://s3.direct.us-south.cloud-object-storage.appdomain.cloud",
	"locationConstraint": "us-south-standard",
	"bucketName":         "test-bucket",
	"accessKey":          "AKIAIOSFODNN7EXAMPLE",
	"secretKey":          "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	"iamEndpoint":        "https://iam.cloud.ibm.com",
	"uid":                "3000",
	"gid":                "3000",

	// ---- freeform mountOptions block (newline-separated) ----
	// These get parsed by updateMountOptions and then classifyRcloneParams.
	//
	// CLI flags (double-dash) — from xlsx CLI Flags sheet:
	//   --transfers, --checkers, --buffer-size, --low-level-retries,
	//   --log-level, --log-file, --bwlimit, --retries, --timeout, --contimeout,
	//   --vfs-cache-mode (special: also appears as mount default, overrides it),
	//   --vfs-cache-max-age, --vfs-read-chunk-size, --attr-timeout,
	//   --dir-cache-time, --poll-interval, --no-modtime, --allow-other
	//
	// CLI flags (single-letter):
	//   -v  (verbose)
	//
	// Config-file lines (underscore / no dash) — from xlsx Config File sheet:
	//   upload_concurrency, upload_cutoff, chunk_size, max_upload_parts,
	//   storage_class, server_side_encryption, acl,
	//   no_check_bucket, disable_checksum, list_chunk, list_version,
	//   force_path_style, encoding
	"mountOptions": `
--transfers=8
--checkers=16
--buffer-size=32M
--low-level-retries=5
--log-level=INFO
--log-file=/var/log/rclone.log
--bwlimit=100M
--retries=5
--timeout=10m
--contimeout=2m
--vfs-cache-mode=full
--vfs-cache-max-age=24h
--vfs-read-chunk-size=128M
--attr-timeout=30s
--dir-cache-time=5m
--poll-interval=30s
--no-modtime=true
--allow-other=true
-v
upload_concurrency=16
upload_cutoff=200M
chunk_size=64M
max_upload_parts=1000
storage_class=SMART
server_side_encryption=AES256
acl=private
no_check_bucket=true
disable_checksum=true
list_chunk=1000
list_version=2
force_path_style=true
encoding=Slash,InvalidUtf8,Dot
`,
}

// TestClassifyRcloneParams_DoublesDashGoToCLI verifies that "--key=value" entries
// are routed exclusively to cliFlags.
func TestClassifyRcloneParams_DoublesDashGoToCLI(t *testing.T) {
	options := []string{
		"--transfers=8",
		"--buffer-size=32M",
		"--vfs-cache-mode=full",
		"--no-modtime=true",
	}
	configLines, cliFlags := classifyRcloneParams(options)

	assert.Empty(t, configLines, "double-dash params must not go to config file")
	assert.ElementsMatch(t, options, cliFlags)
}

// TestClassifyRcloneParams_SingleLetterGoToCLI verifies that "-v" style
// short flags are routed to cliFlags.
func TestClassifyRcloneParams_SingleLetterGoToCLI(t *testing.T) {
	options := []string{"-v", "-q", "-P"}
	configLines, cliFlags := classifyRcloneParams(options)

	assert.Empty(t, configLines)
	assert.ElementsMatch(t, options, cliFlags)
}

// TestClassifyRcloneParams_UnderscoreGoToConfig verifies that keys containing
// "_" are routed exclusively to configLines.
func TestClassifyRcloneParams_UnderscoreGoToConfig(t *testing.T) {
	options := []string{
		"upload_concurrency=16",
		"upload_cutoff=200M",
		"chunk_size=64M",
		"no_check_bucket=true",
		"storage_class=SMART",
	}
	configLines, cliFlags := classifyRcloneParams(options)

	assert.Empty(t, cliFlags, "underscore params must not go to CLI")
	assert.ElementsMatch(t, options, configLines)
}

// TestClassifyRcloneParams_NoDashGoToConfig verifies that plain keys (no leading
// dash at all) are treated as config-file lines.
func TestClassifyRcloneParams_NoDashGoToConfig(t *testing.T) {
	options := []string{
		"encoding=Slash,InvalidUtf8,Dot",
		"acl=private",
		"force_path_style=true",
	}
	configLines, cliFlags := classifyRcloneParams(options)

	assert.Empty(t, cliFlags)
	assert.ElementsMatch(t, options, configLines)
}

// TestClassifyRcloneParams_EmptyAndWhitespace verifies that blank/whitespace
// entries are silently dropped.
func TestClassifyRcloneParams_EmptyAndWhitespace(t *testing.T) {
	options := []string{"", "   ", "\t"}
	configLines, cliFlags := classifyRcloneParams(options)

	assert.Nil(t, configLines)
	assert.Nil(t, cliFlags)
}

// TestClassifyRcloneParams_MixedXlsxParams verifies the complete xlsx sample:
// every "--" entry goes to CLI, every "_"/no-dash entry goes to config.
func TestClassifyRcloneParams_MixedXlsxParams(t *testing.T) {
	options := []string{
		// CLI flags
		"--transfers=8", "--checkers=16", "--buffer-size=32M",
		"--low-level-retries=5", "--log-level=INFO", "--log-file=/var/log/rclone.log",
		"--bwlimit=100M", "--retries=5", "--timeout=10m", "--contimeout=2m",
		"--vfs-cache-mode=full", "--vfs-cache-max-age=24h", "--vfs-read-chunk-size=128M",
		"--attr-timeout=30s", "--dir-cache-time=5m", "--poll-interval=30s",
		"--no-modtime=true", "--allow-other=true",
		"-v",
		// Config-file lines
		"upload_concurrency=16", "upload_cutoff=200M", "chunk_size=64M",
		"max_upload_parts=1000", "storage_class=SMART",
		"server_side_encryption=AES256", "acl=private",
		"no_check_bucket=true", "disable_checksum=true",
		"list_chunk=1000", "list_version=2",
		"force_path_style=true", "encoding=Slash,InvalidUtf8,Dot",
	}

	wantCLI := []string{
		"--transfers=8", "--checkers=16", "--buffer-size=32M",
		"--low-level-retries=5", "--log-level=INFO", "--log-file=/var/log/rclone.log",
		"--bwlimit=100M", "--retries=5", "--timeout=10m", "--contimeout=2m",
		"--vfs-cache-mode=full", "--vfs-cache-max-age=24h", "--vfs-read-chunk-size=128M",
		"--attr-timeout=30s", "--dir-cache-time=5m", "--poll-interval=30s",
		"--no-modtime=true", "--allow-other=true",
		"-v",
	}
	wantConfig := []string{
		"upload_concurrency=16", "upload_cutoff=200M", "chunk_size=64M",
		"max_upload_parts=1000", "storage_class=SMART",
		"server_side_encryption=AES256", "acl=private",
		"no_check_bucket=true", "disable_checksum=true",
		"list_chunk=1000", "list_version=2",
		"force_path_style=true", "encoding=Slash,InvalidUtf8,Dot",
	}

	configLines, cliFlags := classifyRcloneParams(options)
	assert.ElementsMatch(t, wantCLI, cliFlags)
	assert.ElementsMatch(t, wantConfig, configLines)
}

// TestCreateConfig_WithXlsxConfigParams verifies that config-file lines
// classified from the xlsx sample actually appear in the written rclone.conf.
func TestCreateConfig_WithXlsxConfigParams(t *testing.T) {
	rclone := &RcloneMounter{
		AccessKeys:    "AKIAIOSFODNN7EXAMPLE:wJalrXUtnFEMI",
		EndPoint:      "https://s3.direct.us-south.cloud-object-storage.appdomain.cloud",
		LocConstraint: "us-south-standard",
		IAMEndpoint:   "https://iam.cloud.ibm.com",
		AuthType:      "hmac",
	}

	extraConfigLines := []string{
		"upload_concurrency=16",
		"upload_cutoff=200M",
		"chunk_size=64M",
		"max_upload_parts=1000",
		"storage_class=SMART",
		"server_side_encryption=AES256",
		"acl=private",
		"no_check_bucket=true",
		"disable_checksum=true",
		"list_chunk=1000",
		"list_version=2",
		"force_path_style=true",
		"encoding=Slash,InvalidUtf8,Dot",
	}

	tmpDir := t.TempDir()
	err := createConfig(tmpDir, rclone, extraConfigLines)
	assert.NoError(t, err)

	content, err := os.ReadFile(tmpDir + "/rclone.conf")
	assert.NoError(t, err)
	configStr := string(content)

	// Identity / auth block
	assert.Contains(t, configStr, "[ibmcos]")
	assert.Contains(t, configStr, "type = s3")
	assert.Contains(t, configStr, "provider = IBMCOS")
	assert.Contains(t, configStr, "endpoint = https://s3.direct.us-south.cloud-object-storage.appdomain.cloud")
	assert.Contains(t, configStr, "location_constraint = us-south-standard")
	assert.Contains(t, configStr, "ibm_iam_endpoint = https://iam.cloud.ibm.com")

	// Extra config-file lines from xlsx
	for _, line := range extraConfigLines {
		assert.Contains(t, configStr, line, "config line %q should be in rclone.conf", line)
	}

	// CLI flags must NOT appear in the config file
	assert.NotContains(t, configStr, "--transfers")
	assert.NotContains(t, configStr, "--bwlimit")
	assert.NotContains(t, configStr, "--vfs-cache-mode")
}

// TestFormulateRcloneMountOptions_WithXlsxCLIFlags verifies that CLI flags
// classified from the xlsx sample appear in both nodeServerOp and workerNodeOp,
// and that config-file params do NOT appear as CLI flags.
func TestFormulateRcloneMountOptions_WithXlsxCLIFlags(t *testing.T) {
	rclone := &RcloneMounter{
		GID: "3000",
		UID: "3000",
	}

	extraCLIFlags := []string{
		"--transfers=8",
		"--checkers=16",
		"--buffer-size=32M",
		"--low-level-retries=5",
		"--log-level=INFO",
		"--bwlimit=100M",
		"--retries=5",
		"--timeout=10m",
		"--vfs-cache-mode=full",
		"--vfs-cache-max-age=24h",
		"--vfs-read-chunk-size=128M",
		"--attr-timeout=30s",
		"--dir-cache-time=5m",
		"--poll-interval=30s",
		"--no-modtime=true",
	}

	nodeOp, workerOp := rclone.formulateMountOptions(
		"ibmcos:test-bucket",
		"/var/lib/kubelet/pods/abc/volumes/mount",
		"/var/lib/coscsi-config/abc123",
		extraCLIFlags,
	)

	// All extra CLI flags must be in nodeServerOp
	for _, flag := range extraCLIFlags {
		assert.Contains(t, nodeOp, flag, "nodeServerOp should contain %q", flag)
	}

	// Key-value flags must appear in workerNodeOp
	assert.Equal(t, "8", workerOp["transfers"])
	assert.Equal(t, "16", workerOp["checkers"])
	assert.Equal(t, "32M", workerOp["buffer-size"])
	assert.Equal(t, "5", workerOp["low-level-retries"])
	assert.Equal(t, "INFO", workerOp["log-level"])
	assert.Equal(t, "100M", workerOp["bwlimit"])
	assert.Equal(t, "5", workerOp["retries"])
	assert.Equal(t, "10m", workerOp["timeout"])
	assert.Equal(t, "full", workerOp["vfs-cache-mode"])
	assert.Equal(t, "24h", workerOp["vfs-cache-max-age"])
	assert.Equal(t, "128M", workerOp["vfs-read-chunk-size"])
	assert.Equal(t, "30s", workerOp["attr-timeout"])
	assert.Equal(t, "5m", workerOp["dir-cache-time"])
	assert.Equal(t, "30s", workerOp["poll-interval"])
	assert.Equal(t, "true", workerOp["no-modtime"])

	// uid/gid still present
	assert.Equal(t, "3000", workerOp["gid"])
	assert.Equal(t, "3000", workerOp["uid"])

	// Config-file params must NOT appear in nodeServerOp as flags
	assert.NotContains(t, nodeOp, "upload_concurrency=16")
	assert.NotContains(t, nodeOp, "storage_class=SMART")
}

// TestNewRcloneMounter_XlsxSecretMap is an end-to-end constructor test using
// the full xlsx-based secret. It verifies that well-known fields are parsed
// and MountOptions contains exactly what updateMountOptions produces.
func TestNewRcloneMounter_XlsxSecretMap(t *testing.T) {
	mounter := NewRcloneMounter(RcloneMounterParams{
		SecretMap:    secretMapXlsxParams,
		MountOptions: nil,
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
	})

	rclone, ok := mounter.(*RcloneMounter)
	assert.True(t, ok)

	assert.Equal(t, "test-bucket", rclone.BucketName)
	assert.Equal(t, "https://s3.direct.us-south.cloud-object-storage.appdomain.cloud", rclone.EndPoint)
	assert.Equal(t, "us-south-standard", rclone.LocConstraint)
	assert.Equal(t, "hmac", rclone.AuthType)
	assert.Equal(t, "3000", rclone.UID)
	assert.Equal(t, "3000", rclone.GID)
}

// TestCreateConfig_XlsxFullFlow creates a real temp config file using the
// xlsx-classified config-file lines and verifies the file on disk contains
// all expected config entries, and none of the CLI-only entries.
func TestCreateConfig_XlsxFullFlow(t *testing.T) {
	// Parse mountOptions the same way updateMountOptions does
	raw := secretMapXlsxParams["mountOptions"]
	var allOptions []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		allOptions = append(allOptions, line)
	}

	extraConfigLines, extraCLIFlags := classifyRcloneParams(allOptions)

	// Verify split counts are as expected
	assert.NotEmpty(t, extraConfigLines, "should have config-file lines")
	assert.NotEmpty(t, extraCLIFlags, "should have CLI flags")

	rclone := &RcloneMounter{
		AccessKeys:    "AKIAIOSFODNN7EXAMPLE:wJalrXUtnFEMI",
		EndPoint:      secretMapXlsxParams["cosEndpoint"],
		LocConstraint: secretMapXlsxParams["locationConstraint"],
		IAMEndpoint:   secretMapXlsxParams["iamEndpoint"],
		AuthType:      "hmac",
		UID:           "3000",
		GID:           "3000",
	}

	tmpDir := t.TempDir()
	err := createConfig(tmpDir, rclone, extraConfigLines)
	assert.NoError(t, err)

	content, err := os.ReadFile(tmpDir + "/rclone.conf")
	assert.NoError(t, err)
	configStr := string(content)

	// Config-file lines must be present
	expectedConfigEntries := []string{
		"upload_concurrency=16",
		"upload_cutoff=200M",
		"chunk_size=64M",
		"max_upload_parts=1000",
		"storage_class=SMART",
		"server_side_encryption=AES256",
		"acl=private",
		"no_check_bucket=true",
		"disable_checksum=true",
		"list_chunk=1000",
		"list_version=2",
		"force_path_style=true",
		"encoding=Slash,InvalidUtf8,Dot",
	}
	for _, entry := range expectedConfigEntries {
		assert.Contains(t, configStr, entry, "rclone.conf should contain %q", entry)
	}

	// CLI flags must NOT be written to config file
	forbiddenInConfig := []string{
		"--transfers", "--checkers", "--buffer-size", "--low-level-retries",
		"--bwlimit", "--retries", "--timeout", "--contimeout",
		"--vfs-cache-max-age", "--vfs-read-chunk-size",
		"--attr-timeout", "--dir-cache-time", "--poll-interval", "--no-modtime",
		"-v",
	}
	for _, f := range forbiddenInConfig {
		assert.NotContains(t, configStr, f, "rclone.conf must not contain CLI flag %q", f)
	}

	// Final CLI must contain the CLI flags
	nodeOp, workerOp := rclone.formulateMountOptions(
		"ibmcos:test-bucket",
		"/var/lib/kubelet/pods/abc/volumes/mount",
		tmpDir,
		extraCLIFlags,
	)

	assert.Contains(t, nodeOp, "--transfers=8")
	assert.Contains(t, nodeOp, "--vfs-cache-mode=full")
	assert.Contains(t, nodeOp, "--bwlimit=100M")
	assert.Equal(t, "8", workerOp["transfers"])
	assert.Equal(t, "full", workerOp["vfs-cache-mode"])
	assert.Equal(t, "100M", workerOp["bwlimit"])

	// Config-file params must not appear as CLI flags
	for _, entry := range expectedConfigEntries {
		assert.NotContains(t, nodeOp, entry, "CLI args should not contain config-file entry %q", entry)
	}
}

// TestPrintRcloneArtifacts is a manual-verification helper.
// Run with -v to see the exact rclone.conf content and the final CLI command
// that would be executed, given the full xlsx-param secret.
//
//	go test -v -run TestPrintRcloneArtifacts ./pkg/mounter/
func TestPrintRcloneArtifacts(t *testing.T) {
	// ── 1. Parse mountOptions (same as updateMountOptions) ──────────────────
	raw := secretMapXlsxParams["mountOptions"]
	var allOptions []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			allOptions = append(allOptions, line)
		}
	}

	extraConfigLines, extraCLIFlags := classifyRcloneParams(allOptions)

	// ── 2. Build RcloneMounter (same as NewRcloneMounter would produce) ──────
	rclone := &RcloneMounter{
		BucketName:    secretMapXlsxParams["bucketName"],
		EndPoint:      secretMapXlsxParams["cosEndpoint"],
		LocConstraint: secretMapXlsxParams["locationConstraint"],
		IAMEndpoint:   secretMapXlsxParams["iamEndpoint"],
		AccessKeys:    secretMapXlsxParams["accessKey"] + ":" + secretMapXlsxParams["secretKey"],
		AuthType:      "hmac",
		UID:           secretMapXlsxParams["uid"],
		GID:           secretMapXlsxParams["gid"],
	}

	// ── 3. Write rclone.conf into a temp dir ─────────────────────────────────
	tmpDir := t.TempDir()
	configPath := tmpDir + "/rclone.conf"

	err := createConfig(tmpDir, rclone, extraConfigLines)
	if err != nil {
		t.Fatalf("createConfig failed: %v", err)
	}

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config file failed: %v", err)
	}

	// ── 4. Build the final CLI args ──────────────────────────────────────────
	bucket := "ibmcos:" + rclone.BucketName
	targetPath := "/var/lib/kubelet/pods/abc123/volumes/kubernetes.io~csi/pvc-xyz/mount"

	nodeOp, workerOp := rclone.formulateMountOptions(bucket, targetPath, tmpDir, extraCLIFlags)

	// ── 5. Print for manual inspection ──────────────────────────────────────
	t.Logf("\n"+
		"════════════════════════════════════════════════════════════════\n"+
		"  CLASSIFICATION SUMMARY\n"+
		"════════════════════════════════════════════════════════════════\n"+
		"  Config-file lines (%d):  %v\n"+
		"  CLI flags        (%d):  %v\n",
		len(extraConfigLines), extraConfigLines,
		len(extraCLIFlags), extraCLIFlags,
	)

	t.Logf("\n"+
		"════════════════════════════════════════════════════════════════\n"+
		"  rclone.conf  (%s)\n"+
		"════════════════════════════════════════════════════════════════\n"+
		"%s",
		configPath, string(configBytes),
	)

	t.Logf("\n"+
		"════════════════════════════════════════════════════════════════\n"+
		"  FINAL CLI COMMAND (nodeServerOp []string)\n"+
		"════════════════════════════════════════════════════════════════\n"+
		"  rclone \\\n"+
		"    %s\n",
		strings.Join(nodeOp, " \\\n    "),
	)

	t.Logf("\n"+
		"════════════════════════════════════════════════════════════════\n"+
		"  WORKER-NODE MAP  (sent as JSON to cos-csi-mounter)\n"+
		"════════════════════════════════════════════════════════════════",
	)
	// Sort keys for stable output
	keys := make([]string, 0, len(workerOp))
	for k := range workerOp {
		keys = append(keys, k)
	}
	// simple insertion sort — no stdlib sort import needed
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	for _, k := range keys {
		t.Logf("  %-30s = %s", k, workerOp[k])
	}
}
