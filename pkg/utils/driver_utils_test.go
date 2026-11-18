package utils

import (
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/stretchr/testify/assert"
)

func TestGetEndpoints_EnvVarsSet(t *testing.T) {
	customIAM := "https://custom.iam.example.com"
	customCOS := "https://custom.cos.example.com/v1"

	t.Setenv(constants.IAMEndpointEnv, customIAM)
	t.Setenv(constants.COSResourceConfigEndpointEnv, customCOS)

	su := &DriverStatsUtils{}
	iamEP, cosEP, err := su.GetEndpoints()

	assert.NoError(t, err)
	assert.Equal(t, customIAM, iamEP)
	assert.Equal(t, customCOS, cosEP)
}

func TestGetEndpoints_EnvVarsNotSet(t *testing.T) {
	t.Setenv(constants.IAMEndpointEnv, "")
	t.Setenv(constants.COSResourceConfigEndpointEnv, "")

	su := &DriverStatsUtils{}
	iamEP, cosEP, err := su.GetEndpoints()

	assert.NoError(t, err)
	assert.Equal(t, constants.PrivateIAMEndpoint, iamEP)
	assert.Equal(t, constants.ResourceConfigEPDirect, cosEP)
}

func TestGetEndpoints_OnlyIAMEnvVarSet(t *testing.T) {
	customIAM := "https://custom.iam.example.com"

	t.Setenv(constants.IAMEndpointEnv, customIAM)
	t.Setenv(constants.COSResourceConfigEndpointEnv, "")

	su := &DriverStatsUtils{}
	iamEP, cosEP, err := su.GetEndpoints()

	assert.NoError(t, err)
	assert.Equal(t, customIAM, iamEP)
	assert.Equal(t, constants.ResourceConfigEPDirect, cosEP)
}

func TestGetEndpoints_OnlyCOSEnvVarSet(t *testing.T) {
	customCOS := "https://custom.cos.example.com/v1"

	t.Setenv(constants.IAMEndpointEnv, "")
	t.Setenv(constants.COSResourceConfigEndpointEnv, customCOS)

	su := &DriverStatsUtils{}
	iamEP, cosEP, err := su.GetEndpoints()

	assert.NoError(t, err)
	assert.Equal(t, constants.PrivateIAMEndpoint, iamEP)
	assert.Equal(t, customCOS, cosEP)
}

func TestValidateBucketName(t *testing.T) {
	validBuckets := []string{
		"my-bucket",
		"bucket-123",
		"my.bucket.name",
		"123-bucket",
		"balraj-cos",
		"testBucket",
		"a-b-c-1-2-3",
	}
	for _, b := range validBuckets {
		err := ValidateBucketName(b)
		assert.NoError(t, err, "expected valid bucket name: %s", b)
	}

	invalidBuckets := []string{
		"",
		"/etc",
		"../etc",
		"/node-etc",
		"etc/passwd",
		"my..bucket",
		"my.-bucket",
		"my-.bucket",
		"-leading-hyphen",
		"trailing-hyphen-",
		".leading-dot",
		"trailing-dot.",
		"ab", // < 3 chars
		"192.168.1.1",
		"this-bucket-name-is-way-too-long-and-exceeds-the-maximum-sixty-three-characters-limit",
	}
	for _, b := range invalidBuckets {
		err := ValidateBucketName(b)
		assert.Error(t, err, "expected invalid bucket name: %s", b)
	}
}

func TestValidateObjectPath(t *testing.T) {
	validPaths := []string{
		"",
		"folder",
		"sub/folder",
		"path/to/my/file.txt",
		"images/2026/test.png",
	}
	for _, p := range validPaths {
		err := ValidateObjectPath(p)
		assert.NoError(t, err, "expected valid object path: %s", p)
	}

	invalidPaths := []string{
		"../etc",
		"folder/../../etc",
		"folder/../",
		"/../etc",
		"path\x00withnull",
	}
	for _, p := range invalidPaths {
		err := ValidateObjectPath(p)
		assert.Error(t, err, "expected invalid object path: %s", p)
	}
}
