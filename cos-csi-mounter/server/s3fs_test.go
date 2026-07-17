package main

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testBucket         = "testBucket"
	testTargetPath     = "/var/data/kubelet/pods/"
	testEndPoint       = "testEndPoint"
	testIAMEndpoint    = "https://test.iam.cloud.ibm.com"
	testPasswdFilePath = "testPasswdFilePath"
	testURL            = "https://testURL"
)

func TestS3FSPopulateArgsSlice_Success(t *testing.T) {
	args := S3FSArgs{
		AllowOther:     "true",
		EndPoint:       "testEndPoint",
		IBMIamAuth:     "true",
		IBMIamEndpoint: testIAMEndpoint,
	}

	resp, err := args.PopulateArgsSlice(testBucket, testTargetPath)
	assert.NoError(t, err)
	expectedVal := []string{testBucket, testTargetPath, "-o", "ibm_iam_auth", "-o", "ibm_iam_endpoint=" + testIAMEndpoint, "-o", "allow_other", "-o", "endpoint=" + testEndPoint}
	slices.Sort(expectedVal)
	slices.Sort(resp)
	assert.Equal(t, expectedVal, resp)
}

func TestS3FSValidate_Success(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := S3FSArgs{
		PasswdFilePath: testPasswdFilePath,
		URL:            testURL,
	}
	err := args.Validate(testTargetPath)
	assert.NoError(t, err)
}

func TestS3FSValidate_PathValidatorFailed(t *testing.T) {
	args := S3FSArgs{}
	err := args.Validate("invalid-path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value for target path")
}

func TestS3FSValidate_InvalidS3FSParamValues(t *testing.T) {
	fields := []string{
		"AllowOther",
		"AutoCache",
		"ConnectTimeoutSeconds",
		"CurlDebug",
		"GID",
		"IBMIamAuth",
		"IBMIamEndpoint",
		"KernelCache",
		"MaxBackground",
		"MaxDirtyData",
		"MaxStatCacheSize",
		"MultiPartSize",
		"MultiReqMax",
		"ParallelCount",
		"ReadOnly",
		"ReadwriteTimeoutSeconds",
		"RetryCount",
		"SigV2",
		"SigV4",
		"StatCacheExpireSeconds",
		"UID",
		"UsePathRequestStyle",
		"UseXattr",
		"URL",
	}
	for _, f := range fields {
		args := S3FSArgs{}

		val := reflect.ValueOf(&args).Elem().FieldByName(f)
		val.SetString("invalid-value")
		err := args.Validate(testTargetPath)
		assert.Error(t, err)
	}
}

func TestS3FSValidate_FailedToCheckPasswordFile(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return false, errors.New("error")
	}
	args := S3FSArgs{
		PasswdFilePath: testPasswdFilePath,
	}
	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error checking credential file existence")
}

func TestS3FSValidate_PasswordFileNotFound(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return false, nil
	}

	args := S3FSArgs{
		PasswdFilePath: testPasswdFilePath,
	}

	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credential file not found")
}

func TestS3FSValidate_RetryCountLessThanOne(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := S3FSArgs{
		PasswdFilePath: testPasswdFilePath,
		RetryCount:     "0",
	}

	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "value of retires should be >= 1")
}

func TestS3FSValidate_StatCacheExpireSecondsThanZero(t *testing.T) {
	FileExists = func(path string) (bool, error) {
		return true, nil
	}

	args := S3FSArgs{
		PasswdFilePath:         testPasswdFilePath,
		StatCacheExpireSeconds: "-1",
	}

	err := args.Validate(testTargetPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "value of stat_cache_expire should be >= 0")
}

func TestS3FSPopulateArgsSlice_SkipCipherSuitesDefault(t *testing.T) {
	tests := []struct {
		name         string
		args         S3FSArgs
		expectAbsent string
		expectPresent string
	}{
		{
			name:         "cipher_suites=default is not passed to s3fs",
			args:         S3FSArgs{CipherSuites: "default", PasswdFilePath: testPasswdFilePath, URL: testURL},
			expectAbsent: "cipher_suites=default",
		},
		{
			name:          "cipher_suites with non-default value is passed to s3fs",
			args:          S3FSArgs{CipherSuites: "AESGCM", PasswdFilePath: testPasswdFilePath, URL: testURL},
			expectPresent: "cipher_suites=AESGCM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.args.PopulateArgsSlice(testBucket, testTargetPath)
			assert.NoError(t, err)
			resultStr := strings.Join(result, " ")
			if tt.expectAbsent != "" {
				assert.NotContains(t, resultStr, tt.expectAbsent,
					"cipher_suites=default must not be forwarded to s3fs command")
			}
			if tt.expectPresent != "" {
				assert.Contains(t, resultStr, tt.expectPresent)
			}
		})
	}
}
