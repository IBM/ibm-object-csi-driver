package mounter

import (
	"reflect"
	"sort"
	"testing"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/stretchr/testify/assert"
)

func stringSlicesEqualIgnoreOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))
	copy(aCopy, a)
	copy(bCopy, b)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

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
				"objectPath":         "test-obj-path",
				"accessKey":          "test-access-key",
				"secretKey":          "test-secret-key",
				"apiKey":             "test-api-key",
				"kpRootKeyCRN":       "test-kp-root-key-crn",
			},
			mountOptions: []string{"opt1=val1", "cipher_suites=default"},
			expected: &S3fsMounter{
				BucketName:    "test-bucket-name",
				objectPath:    "test-obj-path",
				EndPoint:      "test-endpoint",
				LocConstraint: "test-loc-constraint",
				AccessKeys:    ":test-api-key",
				AuthType:      "iam",
				KpRootKeyCrn:  "test-kp-root-key-crn",
				MountOptions:  []string{"opt1=val1", "cipher_suites=default"},
				MounterUtils:  &mounterUtils.MounterOptsUtils{},
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
				"objectPath":         "test-obj-path",
				"accessKey":          "test-access-key",
				"secretKey":          "test-secret-key",
				"kpRootKeyCRN":       "test-kp-root-key-crn",
				"gid":                "fake-gid",
				"uid":                "fake-uid",
			},
			mountOptions: []string{"opt1=val1", "opt2=val2"},
			expected: &RcloneMounter{
				BucketName:    "test-bucket-name",
				objectPath:    "test-obj-path",
				EndPoint:      "test-endpoint",
				LocConstraint: "test-loc-constraint",
				AccessKeys:    "test-access-key:test-secret-key",
				AuthType:      "hmac",
				KpRootKeyCrn:  "test-kp-root-key-crn",
				UID:           "fake-uid",
				GID:           "fake-gid",
				MountOptions:  []string{"opt1=val1", "opt2=val2"},
				MounterUtils:  &mounterUtils.MounterOptsUtils{},
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
				"objectPath":         "test-obj-path",
				"accessKey":          "test-access-key",
				"secretKey":          "test-secret-key",
				"kpRootKeyCRN":       "test-kp-root-key-crn",
			},
			mountOptions: []string{"cipher_suites=default"},
			expected: &S3fsMounter{
				BucketName:    "test-bucket-name",
				objectPath:    "test-obj-path",
				EndPoint:      "test-endpoint",
				LocConstraint: "test-loc-constraint",
				AccessKeys:    "test-access-key:test-secret-key",
				AuthType:      "hmac",
				KpRootKeyCrn:  "test-kp-root-key-crn",
				MountOptions:  []string{"cipher_suites=default"},
				MounterUtils:  &mounterUtils.MounterOptsUtils{},
			},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factory := &CSIMounterFactory{}

			result := factory.NewMounter(test.attrib, test.secretMap, test.mountOptions, nil)

			if s3fs, ok := result.(*S3fsMounter); ok {
				expected := test.expected.(*S3fsMounter)
				if !stringSlicesEqualIgnoreOrder(s3fs.MountOptions, expected.MountOptions) {
					t.Errorf("MountOptions mismatch.\nGot:  %v\nWant: %v", s3fs.MountOptions, expected.MountOptions)
				}
				s3fs.MountOptions = nil
				expected.MountOptions = nil
			}
			if rclone, ok := result.(*RcloneMounter); ok {
				expected := test.expected.(*RcloneMounter)
				if !stringSlicesEqualIgnoreOrder(rclone.MountOptions, expected.MountOptions) {
					t.Errorf("MountOptions mismatch.\nGot:  %v\nWant: %v", rclone.MountOptions, expected.MountOptions)
				}
				rclone.MountOptions = nil
				expected.MountOptions = nil
			}

			assert.Equal(t, result, test.expected)

			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("Result does not match expected output.\nExpected: %v\nGot: %v", test.expected, result)
			}
		})
	}
}
