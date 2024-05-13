package mounter

import (
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"

	"reflect"
	"testing"
)

func TestNewMounter(t *testing.T) {
	tests := []struct {
		name         string
		attrib       map[string]string
		secretMap    map[string]string
		mountOptions []string
		expected     Mounter
		expectedErr  error
	}{
		{
			name:   "S3fs Mounter",
			attrib: map[string]string{"mounter": constants.S3FS},
			secretMap: map[string]string{
				"cosEndpoint":        "test-endpoint",
				"locationConstraint": "test-loc-constraint",
				"bucketName":         "test-bucket-name",
				"objPath":            "test-obj-path",
				"accessKey":          "test-access-key",
				"secretKey":          "test-secret-key",
				"apiKey":             "test-api-key",
				"kpRootKeyCRN":       "test-kp-root-key-crn",
			},
			mountOptions: []string{"opt1=val1"},
			expected: &S3fsMounter{
				BucketName:    "test-bucket-name",
				ObjPath:       "test-obj-path",
				EndPoint:      "test-endpoint",
				LocConstraint: "test-loc-constraint",
				AccessKeys:    ":test-api-key",
				AuthType:      "iam",
				KpRootKeyCrn:  "test-kp-root-key-crn",
				MountOptions:  []string{"opt1=val1"},
				MounterUtils:  &(mounterUtils.MounterOptsUtils{}),
			},
			expectedErr: nil,
		},
		{
			name:   "Rclone Mounter",
			attrib: map[string]string{"mounter": constants.RClone},
			secretMap: map[string]string{
				"cosEndpoint":        "test-endpoint",
				"locationConstraint": "test-loc-constraint",
				"bucketName":         "test-bucket-name",
				"objPath":            "test-obj-path",
				"accessKey":          "test-access-key",
				"secretKey":          "test-secret-key",
				"kpRootKeyCRN":       "test-kp-root-key-crn",
				"gid":                "fake-gid",
				"uid":                "fake-uid",
			},
			mountOptions: []string{"opt1=val1", "opt2=val2"},
			expected: &RcloneMounter{
				BucketName:    "test-bucket-name",
				ObjPath:       "test-obj-path",
				EndPoint:      "test-endpoint",
				LocConstraint: "test-loc-constraint",
				AccessKeys:    "test-access-key:test-secret-key",
				AuthType:      "hmac",
				KpRootKeyCrn:  "test-kp-root-key-crn",
				UID:           "fake-uid",
				GID:           "fake-gid",
				MountOptions:  []string{"opt1=val1", "opt2=val2"},
				MounterUtils:  &(mounterUtils.MounterOptsUtils{}),
			},
			expectedErr: nil,
		},
		{
			name:   "Default Mounter",
			attrib: map[string]string{},
			secretMap: map[string]string{
				"cosEndpoint":        "test-endpoint",
				"locationConstraint": "test-loc-constraint",
				"bucketName":         "test-bucket-name",
				"objPath":            "test-obj-path",
				"accessKey":          "test-access-key",
				"secretKey":          "test-secret-key",
				"kpRootKeyCRN":       "test-kp-root-key-crn",
			},
			mountOptions: []string{},
			expected: &S3fsMounter{
				BucketName:    "test-bucket-name",
				ObjPath:       "test-obj-path",
				EndPoint:      "test-endpoint",
				LocConstraint: "test-loc-constraint",
				AccessKeys:    "test-access-key:test-secret-key",
				AuthType:      "hmac",
				KpRootKeyCrn:  "test-kp-root-key-crn",
				MountOptions:  []string{},
				MounterUtils:  &(mounterUtils.MounterOptsUtils{}),
			},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			factory := &S3fsMounterFactory{}

			result, err := factory.NewMounter(test.attrib, test.secretMap, test.mountOptions)

			if err != test.expectedErr {
				t.Errorf("Expected error: %v, but got: %v", test.expectedErr, err)
			}

			assert.Equal(t, result, test.expected)

			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("Result does not match expected output.\nExpected: %v\nGot: %v", test.expected, result)
			}
		})
	}
}
