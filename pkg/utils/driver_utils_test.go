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
