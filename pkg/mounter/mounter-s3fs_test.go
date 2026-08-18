package mounter

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

var (
	secretMap = map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"apiKey":             "test-api-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"uid":                "test-uid",
	}

	mountOptions = []string{"opt1=val1", "opt2=val2", "opt3"}
)

func TestNewS3fsMounter_Success(t *testing.T) {
	mounter := NewS3fsMounter(S3fsMounterParams{
		SecretMap:        secretMap,
		MountOptions:     mountOptions,
		MounterUtils:     mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
		KnownS3FSOptions: GetKnownS3FSOptions(),
		DefaultParams:    map[string]string{constants.CipherSuitesKey: "default"},
	})

	s3fsMounter, ok := mounter.(*S3fsMounter)
	assert.True(t, ok)

	assert.Equal(t, s3fsMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, s3fsMounter.ObjectPath, secretMap["objectPath"])
	assert.Equal(t, s3fsMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, s3fsMounter.LocConstraint, secretMap["locationConstraint"])
}

func TestNewS3fsMounter_Success_Hmac(t *testing.T) {
	secretMap := map[string]string{
		"cosEndpoint":        "test-endpoint",
		"locationConstraint": "test-loc-constraint",
		"bucketName":         "test-bucket-name",
		"objectPath":         "test-obj-path",
		"accessKey":          "test-access-key",
		"secretKey":          "test-secret-key",
		"kpRootKeyCRN":       "test-kp-root-key-crn",
		"mountOptions":       "\nkey1\nkey2=value2\nkey=val1=val2",
		"tmpdir":             "test-tmpdir",
		"use_cache":          "true",
		"gid":                "test-gid",
		"iamEndpoint":        "test-iamEndpoint",
	}

	mountOptions := []string{"opt1=val1", "opt2=val2", " ", "opt3"}

	mounter := NewS3fsMounter(S3fsMounterParams{
		SecretMap:        secretMap,
		MountOptions:     mountOptions,
		MounterUtils:     mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{}),
		KnownS3FSOptions: GetKnownS3FSOptions(),
	})

	s3fsMounter, ok := mounter.(*S3fsMounter)
	assert.True(t, ok)

	assert.Equal(t, s3fsMounter.BucketName, secretMap["bucketName"])
	assert.Equal(t, s3fsMounter.ObjectPath, secretMap["objectPath"])
	assert.Equal(t, s3fsMounter.EndPoint, secretMap["cosEndpoint"])
	assert.Equal(t, s3fsMounter.LocConstraint, secretMap["locationConstraint"])
}

func TestS3FSMount_NodeServer_Positive(t *testing.T) {
	mountWorker = false

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return nil
	}

	s3fs := &S3fsMounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
		LocConstraint: "test-location",
		MountOptions:  mountOptions,
		ObjectPath:    "test-objectPath",
	}

	err := s3fs.Mount(source, target)
	assert.NoError(t, err)
}

func TestS3FSMount_WorkerNode_Positive(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return nil
	}

	s3fs := &S3fsMounter{
		MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
			FuseMountFn: func(path, comm string, args []string) error {
				return nil
			},
		}),
		AuthType: "hmac",
	}

	err := s3fs.Mount(source, target)
	assert.NoError(t, err)
}

func TestMount_CreateDirFails_Negative(t *testing.T) {
	s3fs := &S3fsMounter{}

	MakeDir = func(path string, perm os.FileMode) error {
		return errors.New("failed to create directory")
	}

	err := s3fs.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot create directory")
}

func TestMount_FailedToCreatePassFile_Negative(t *testing.T) {
	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return errors.New("failed to create file")
	}

	s3fs := &S3fsMounter{}

	err := s3fs.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create file")
}

func TestS3FSMount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	MakeDir = func(path string, perm os.FileMode) error {
		return nil
	}
	writePassWrap = func(_, _ string) error {
		return nil
	}
	mounterRequest = func(_, _ string) error {
		return errors.New("failed to perform http request")
	}

	s3fs := &S3fsMounter{}

	err := s3fs.Mount(source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to perform http request")
}

func TestUnmount_NodeServer(t *testing.T) {
	mountWorker = false

	removeFile = func(_, _ string) {}

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	err := s3fs.Unmount(target)
	assert.NoError(t, err)
}

func TestUnmount_WorkerNode(t *testing.T) {
	mountWorker = true

	removeFile = func(_, _ string) {}

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return nil
	}

	err := s3fs.Unmount(target)
	assert.NoError(t, err)
}

func TestUnmount_WorkerNode_Negative(t *testing.T) {
	mountWorker = true

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return nil
		},
	})}

	mounterRequest = func(_, _ string) error {
		return errors.New("failed to create http request")
	}

	err := s3fs.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create http request")
}

func TestUnmount_NodeServer_Negative(t *testing.T) {
	mountWorker = false

	removeFile = func(_, _ string) {}

	s3fs := &S3fsMounter{MounterUtils: mounterUtils.NewFakeMounterUtilsImpl(mounterUtils.FakeMounterUtilsFuncStruct{
		FuseUnmountFn: func(path string) error {
			return errors.New("failed to unmount")
		},
	})}

	err := s3fs.Unmount(target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmount")
}

func TestRemoveS3FSCredFile_PathNotExists(t *testing.T) {
	Stat = func(path string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	removeS3FSCredFile("/test", target)
}

func TestRemoveS3FSCredFile_StatRetryThenSuccess(t *testing.T) {
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

	removeS3FSCredFile("/test", target)
}

func TestRemoveS3FSCredFile_RemoveRetryThenSuccess(t *testing.T) {
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

	removeS3FSCredFile("/test", target)
}

func TestRemoveS3FSCredFile_Negative(t *testing.T) {
	called := 0
	Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}
	RemoveAll = func(_ string) error {
		called++
		return errors.New("remove failed")
	}

	removeS3FSCredFile("/test", target)
	assert.Equal(t, maxRetries, called)
}

func TestAddMountParam_Integration(t *testing.T) {
	tests := []struct {
		name           string
		defaultOpts    []string
		secretOpts     string
		wantUnknown    []string
		wantEmpty      bool
	}{
		{"OnlyKnown", []string{"allow_other"}, "kernel_cache", nil, true},
		{"OnlyUnknown", nil, "enable_content_md5\ncomplement_stat", []string{"enable_content_md5", "complement_stat"}, false},
		{"Mixed", []string{"allow_other"}, "enable_content_md5\nkernel_cache", []string{"enable_content_md5"}, false},
		{"SpecialChars", nil, "mime=/etc/mime.types", []string{"mime=/etc/mime.types"}, false},
		{"EmptyLines", nil, "allow_other\n\nenable_content_md5\n", []string{"enable_content_md5"}, false},
		{"UnknownInDefault", []string{"enable_content_md5", "allow_other"}, "", []string{"enable_content_md5"}, false},
		{"NoSecret", []string{"enable_content_md5"}, "", []string{"enable_content_md5"}, false},
		{"InvalidLines", nil, "allow_other\n===\nenable_content_md5", []string{"enable_content_md5"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := map[string]string{}
			if tt.secretOpts != "" {
				secret["mountOptions"] = tt.secretOpts
			}

			opts, addParam := updateS3FSMountOptions(tt.defaultOpts, secret, GetKnownS3FSOptions(), nil, "", false)
			assert.NotNil(t, opts)

			if tt.wantEmpty {
				assert.Empty(t, addParam)
			} else {
				assert.NotEmpty(t, addParam)
				for _, opt := range tt.wantUnknown {
					assert.Contains(t, addParam, opt)
				}
			}
		})
	}
}

func TestFormulateOptions_AddMountParam(t *testing.T) {
	tests := []struct {
		param string
		want  bool
	}{
		{"", false},
		{"-o enable_content_md5", true},
		{"-o enable_content_md5 -o complement_stat", true},
	}

	for _, tt := range tests {
		s3fs := &S3fsMounter{AddMountParam: tt.param, EndPoint: "https://s3.test.com"}
		_, workerOp := s3fs.formulateMountOptions("bucket", "/target", "/passwd")

		_, exists := workerOp["add-mount-param"]
		assert.Equal(t, tt.want, exists)
	}
}

func TestUpdateS3FSMountOptions_SpecialSecretFields(t *testing.T) {
	tests := []struct {
		name        string
		secret      map[string]string
		defaultOpts []string
		wantUID     bool
		wantGID     bool
	}{
		{
			name:    "GID without UID sets both",
			secret:  map[string]string{"gid": "1000"},
			wantUID: true,
			wantGID: true,
		},
		{
			name:    "UID overrides GID",
			secret:  map[string]string{"gid": "1000", "uid": "2000"},
			wantUID: true,
			wantGID: true,
		},
		{
			name:    "tmpdir and use_cache",
			secret:  map[string]string{"tmpdir": "/tmp", "use_cache": "true"},
			wantUID: false,
			wantGID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, _ := updateS3FSMountOptions(tt.defaultOpts, tt.secret, GetKnownS3FSOptions(), nil, "", false)
			assert.NotNil(t, opts)
			
			optsStr := strings.Join(opts, " ")
			if tt.wantUID {
				assert.Contains(t, optsStr, "uid=")
			}
			if tt.wantGID {
				assert.Contains(t, optsStr, "gid=")
			}
		})
	}
}

func TestUpdateS3FSMountOptions_DefaultParams(t *testing.T) {
	defaultParams := map[string]string{
		"cipher_suites": "AESGCM",
		"empty_param":   "",
	}
	
	opts, _ := updateS3FSMountOptions(nil, map[string]string{}, GetKnownS3FSOptions(), defaultParams, "", false)
	
	optsStr := strings.Join(opts, " ")
	assert.Contains(t, optsStr, "cipher_suites=AESGCM")
	assert.NotContains(t, optsStr, "empty_param")
}

func TestUpdateS3FSMountOptions_ReadOnly(t *testing.T) {
	tests := []struct {
		name     string
		readOnly bool
		wantRO   bool
	}{
		{
			name:     "readOnly true sets ro=true",
			readOnly: true,
			wantRO:   true,
		},
		{
			name:     "readOnly false does not set ro",
			readOnly: false,
			wantRO:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, _ := updateS3FSMountOptions(nil, map[string]string{}, GetKnownS3FSOptions(), nil, "", tt.readOnly)
			optsStr := strings.Join(opts, " ")
			if tt.wantRO {
				assert.Contains(t, optsStr, "ro=true")
			} else {
				assert.NotContains(t, optsStr, "ro=true")
			}
		})
	}
}

func TestUpdateS3FSMountOptions_GidParam(t *testing.T) {
	tests := []struct {
		name        string
		secret      map[string]string
		gid         string
		wantGID     string
		wantUID     string
	}{
		{
			name:    "gid param overrides secretMap gid",
			secret:  map[string]string{"gid": "1000"},
			gid:     "2000",
			wantGID: "gid=2000",
			wantUID: "uid=2000", // auto-set from gid param since no uid in secret
		},
		{
			name:    "gid param sets uid when uid absent",
			secret:  map[string]string{},
			gid:     "3000",
			wantGID: "gid=3000",
			wantUID: "uid=3000",
		},
		{
			name:    "gid param does not override explicit secretMap uid",
			secret:  map[string]string{"uid": "5000"},
			gid:     "4000",
			wantGID: "gid=4000",
			wantUID: "uid=5000", // secretMap uid takes precedence
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
			opts, _ := updateS3FSMountOptions(nil, tt.secret, GetKnownS3FSOptions(), nil, tt.gid, false)
			optsStr := strings.Join(opts, " ")
			if tt.wantGID != "" {
				assert.Contains(t, optsStr, tt.wantGID)
			} else {
				assert.NotContains(t, optsStr, "gid=")
			}
			if tt.wantUID != "" {
				assert.Contains(t, optsStr, tt.wantUID)
			} else {
				assert.NotContains(t, optsStr, "uid=")
			}
		})
	}
}

func TestUpdateS3FSMountOptionsWithUnknownOptions(t *testing.T) {
	tests := []struct {
		name                        string
		defaultMountOp              []string
		secretMap                   map[string]string
		expectAddMountParamPresent  bool
		expectAddMountParamContains string
	}{
		{
			name:           "Known and unknown options",
			defaultMountOp: []string{"allow_other", "enable_content_md5=true"},
			secretMap:      map[string]string{"mountOptions": "cipher_suites=default"},
			expectAddMountParamPresent:  true,
			expectAddMountParamContains: "enable_content_md5=true",
		},
		{
			name:           "Only known options",
			defaultMountOp: []string{"allow_other"},
			secretMap:      map[string]string{},
			expectAddMountParamPresent:  false,
			expectAddMountParamContains: "",
		},
		{
			name:           "Unknown option without value",
			defaultMountOp: []string{"custom_flag"},
			secretMap:      map[string]string{},
			expectAddMountParamPresent:  true,
			expectAddMountParamContains: "custom_flag",
		},
		{
			name:           "Multiple unknown options from secretMap",
			defaultMountOp: []string{},
			secretMap:      map[string]string{"mountOptions": "unknown1=val1\nunknown2=val2"},
			expectAddMountParamPresent:  true,
			expectAddMountParamContains: "unknown1=val1",
		},
		{
			name:           "Empty line in mountOptions",
			defaultMountOp: []string{},
			secretMap:      map[string]string{"mountOptions": "allow_other\n\nunknown_opt=test"},
			expectAddMountParamPresent:  true,
			expectAddMountParamContains: "unknown_opt=test",
		},
		{
			name:           "Invalid option in defaultMountOp",
			defaultMountOp: []string{"", "allow_other"},
			secretMap:      map[string]string{},
			expectAddMountParamPresent:  false,
			expectAddMountParamContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, addMountParam := updateS3FSMountOptions(tt.defaultMountOp, tt.secretMap, GetKnownS3FSOptions(), map[string]string{}, "", false)
			hasUnknown := addMountParam != ""
			if hasUnknown != tt.expectAddMountParamPresent {
				t.Errorf("got addMountParam=%q, expectAddMountParamPresent=%v", addMountParam, tt.expectAddMountParamPresent)
			}
			if tt.expectAddMountParamContains != "" && !strings.Contains(addMountParam, tt.expectAddMountParamContains) {
				t.Errorf("addMountParam=%q does not contain %q", addMountParam, tt.expectAddMountParamContains)
			}
		})
	}
}

func TestUpdateS3FSMountOptions_SkipCipherSuitesDefault(t *testing.T) {
	tests := []struct {
		name           string
		defaultMountOp []string
		secretMap      map[string]string
	}{
		{
			name:           "cipher_suites=default in defaultMountOp is dropped",
			defaultMountOp: []string{"cipher_suites=default"},
			secretMap:      map[string]string{},
		},
		{
			name:           "cipher_suites=default in secret mountOptions is dropped",
			defaultMountOp: []string{},
			secretMap:      map[string]string{"mountOptions": "cipher_suites=default"},
		},
		{
			name:           "cipher_suites=default case-insensitive (DEFAULT) is dropped",
			defaultMountOp: []string{"cipher_suites=DEFAULT"},
			secretMap:      map[string]string{},
		},
		{
			name:           "cipher_suites=default dropped but other options retained",
			defaultMountOp: []string{"cipher_suites=default", "dbglevel=warn"},
			secretMap:      map[string]string{},
		},
		{
			name:           "cipher_suites with non-default value is NOT dropped",
			defaultMountOp: []string{"cipher_suites=AESGCM"},
			secretMap:      map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, _ := updateS3FSMountOptions(tt.defaultMountOp, tt.secretMap, GetKnownS3FSOptions(), map[string]string{}, "", false)
			optsStr := strings.Join(opts, " ")

			if strings.Contains(tt.name, "NOT dropped") {
				assert.Contains(t, optsStr, "cipher_suites=AESGCM")
			} else {
				assert.NotContains(t, optsStr, "cipher_suites=default",
					"cipher_suites=default must not be passed to s3fs")
				assert.NotContains(t, optsStr, "cipher_suites=DEFAULT",
					"cipher_suites=DEFAULT must not be passed to s3fs")
			}
		})
	}
}
