/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2026 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package utils

import (
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sensitiveKeys lists every secret field that must never appear in logs.
var sensitiveKeys = []string{
	constants.AccessKey,
	constants.SecretKey,
	constants.ApiKey,
	constants.KpRootKeyCRN,
	constants.ServiceId,
	constants.ResourceConfigApiKey, // "resourceConfigApiKey"
}

// allSecrets is a complete secret map with values matching their key names so
// that any unmasked field is trivially detectable.
var allSecrets = map[string]string{
	constants.AccessKey:            "val-accessKey",
	constants.SecretKey:            "val-secretKey",
	constants.ApiKey:               "val-apiKey",
	constants.KpRootKeyCRN:         "val-kpRootKeyCRN",
	constants.ServiceId:            "val-serviceId",
	constants.ResourceConfigApiKey: "val-resourceConfigApiKey",
	// Non-sensitive fields that must pass through unchanged.
	"cosEndpoint":        "https://s3.us-south.cloud-object-storage.appdomain.cloud",
	"locationConstraint": "us-south-standard",
	"bucketName":         "my-bucket",
	"iamEndpoint":        "https://iam.cloud.ibm.com",
}

// assertSecretsMasked verifies that every sensitive key in got is set to
// "xxxxxxx" and that non-sensitive keys are preserved verbatim.
func assertSecretsMasked(t *testing.T, got map[string]string) {
	t.Helper()
	for _, k := range sensitiveKeys {
		if _, present := allSecrets[k]; !present {
			continue
		}
		assert.Equal(t, "xxxxxxx", got[k], "sensitive key %q must be masked", k)
	}
	// Non-sensitive fields must pass through.
	nonSensitive := []string{"cosEndpoint", "locationConstraint", "bucketName", "iamEndpoint"}
	for _, k := range nonSensitive {
		assert.Equal(t, allSecrets[k], got[k], "non-sensitive key %q must be preserved", k)
	}
}

// --- CreateVolumeRequest ---

func TestReplaceAndReturnCopy_CreateVolume_MasksSensitiveFields(t *testing.T) {
	req := &csi.CreateVolumeRequest{
		Name:    "test-volume",
		Secrets: allSecrets,
	}

	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)

	got, ok := result.(*csi.CreateVolumeRequest)
	require.True(t, ok)
	assertSecretsMasked(t, got.Secrets)

	// Original request must not be mutated.
	assert.Equal(t, "val-accessKey", req.Secrets[constants.AccessKey], "original request must not be mutated")
}

func TestReplaceAndReturnCopy_CreateVolume_EmptySecrets(t *testing.T) {
	req := &csi.CreateVolumeRequest{Name: "test-volume", Secrets: map[string]string{}}
	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)
	got := result.(*csi.CreateVolumeRequest)
	assert.Empty(t, got.Secrets)
}

// --- DeleteVolumeRequest ---

func TestReplaceAndReturnCopy_DeleteVolume_MasksSensitiveFields(t *testing.T) {
	req := &csi.DeleteVolumeRequest{
		VolumeId: "vol-123",
		Secrets:  allSecrets,
	}

	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)

	got, ok := result.(*csi.DeleteVolumeRequest)
	require.True(t, ok)
	assertSecretsMasked(t, got.Secrets)

	assert.Equal(t, "val-secretKey", req.Secrets[constants.SecretKey], "original request must not be mutated")
}

func TestReplaceAndReturnCopy_DeleteVolume_EmptySecrets(t *testing.T) {
	req := &csi.DeleteVolumeRequest{VolumeId: "vol-123", Secrets: map[string]string{}}
	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)
	got := result.(*csi.DeleteVolumeRequest)
	assert.Empty(t, got.Secrets)
}

// --- NodePublishVolumeRequest ---

func TestReplaceAndReturnCopy_NodePublish_MasksSensitiveFields(t *testing.T) {
	req := &csi.NodePublishVolumeRequest{
		VolumeId:   "vol-456",
		TargetPath: "/var/lib/kubelet/pods/pod1/volumes",
		Secrets:    allSecrets,
	}

	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)

	got, ok := result.(*csi.NodePublishVolumeRequest)
	require.True(t, ok)
	assertSecretsMasked(t, got.Secrets)

	assert.Equal(t, "val-apiKey", req.Secrets[constants.ApiKey], "original request must not be mutated")
}

func TestReplaceAndReturnCopy_NodePublish_EmptySecrets(t *testing.T) {
	req := &csi.NodePublishVolumeRequest{VolumeId: "vol-456", Secrets: map[string]string{}}
	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)
	got := result.(*csi.NodePublishVolumeRequest)
	assert.Empty(t, got.Secrets)
}

// --- NodeStageVolumeRequest ---

func TestReplaceAndReturnCopy_NodeStage_MasksSensitiveFields(t *testing.T) {
	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "vol-789",
		StagingTargetPath: "/var/lib/kubelet/plugins/stage",
		Secrets:           allSecrets,
	}

	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)

	got, ok := result.(*csi.NodeStageVolumeRequest)
	require.True(t, ok)
	assertSecretsMasked(t, got.Secrets)

	assert.Equal(t, "val-kpRootKeyCRN", req.Secrets[constants.KpRootKeyCRN], "original request must not be mutated")
}

func TestReplaceAndReturnCopy_NodeStage_EmptySecrets(t *testing.T) {
	req := &csi.NodeStageVolumeRequest{VolumeId: "vol-789", Secrets: map[string]string{}}
	result, err := ReplaceAndReturnCopy(req)
	require.NoError(t, err)
	got := result.(*csi.NodeStageVolumeRequest)
	assert.Empty(t, got.Secrets)
}

// --- ResourceConfigApiKey specifically ---

func TestReplaceAndReturnCopy_ResourceConfigApiKey_Masked(t *testing.T) {
	for _, reqType := range []interface{}{
		&csi.CreateVolumeRequest{Secrets: map[string]string{constants.ResourceConfigApiKey: "super-secret-rc-key"}},
		&csi.DeleteVolumeRequest{Secrets: map[string]string{constants.ResourceConfigApiKey: "super-secret-rc-key"}},
		&csi.NodePublishVolumeRequest{Secrets: map[string]string{constants.ResourceConfigApiKey: "super-secret-rc-key"}},
		&csi.NodeStageVolumeRequest{Secrets: map[string]string{constants.ResourceConfigApiKey: "super-secret-rc-key"}},
	} {
		result, err := ReplaceAndReturnCopy(reqType)
		require.NoError(t, err)

		var secrets map[string]string
		switch r := result.(type) {
		case *csi.CreateVolumeRequest:
			secrets = r.Secrets
		case *csi.DeleteVolumeRequest:
			secrets = r.Secrets
		case *csi.NodePublishVolumeRequest:
			secrets = r.Secrets
		case *csi.NodeStageVolumeRequest:
			secrets = r.Secrets
		}
		assert.Equal(t, "xxxxxxx", secrets[constants.ResourceConfigApiKey],
			"resourceConfigApiKey must be masked in %T", reqType)
	}
}

// --- Unsupported type ---

func TestReplaceAndReturnCopy_UnsupportedType_ReturnsError(t *testing.T) {
	_, err := ReplaceAndReturnCopy(&csi.ListVolumesRequest{})
	assert.Error(t, err)
}
