package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "long volume ID",
			input:    "volume-id-1234567890",
			expected: "vo*********90",
		},
		{
			name:     "short string <= 4 chars",
			input:    "key",
			expected: "****",
		},
		{
			name:     "exactly 4 chars",
			input:    "test",
			expected: "****",
		},
		{
			name:     "exactly 5 chars",
			input:    "test1",
			expected: "te*1",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "****",
		},
		{
			name:     "endpoint URL",
			input:    "https://s3.us-south.cloud-object-storage.appdomain.cloud",
			expected: "ht*********ud",
		},
		{
			name:     "api key",
			input:    "apikey-xyz789",
			expected: "ap*********89",
		},
		{
			name:     "secret key",
			input:    "secret-key-abc123",
			expected: "se*********23",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskSensitive(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMaskParams(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "no sensitive params",
			input:    map[string]string{"bucket": "mybucket", "region": "us-south"},
			expected: map[string]string{"bucket": "mybucket", "region": "us-south"},
		},
		{
			name:     "with access key",
			input:    map[string]string{"accessKey": "mykey123", "bucket": "mybucket"},
			expected: map[string]string{"accessKey": "my*********23", "bucket": "mybucket"},
		},
		{
			name:     "with secret key",
			input:    map[string]string{"secretKey": "secret123", "bucket": "mybucket"},
			expected: map[string]string{"secretKey": "se*********23", "bucket": "mybucket"},
		},
		{
			name:     "with api key",
			input:    map[string]string{"apiKey": "apikey123", "bucket": "mybucket"},
			expected: map[string]string{"apiKey": "ap*********23", "bucket": "mybucket"},
		},
		{
			name:     "with resourceConfigApiKey",
			input:    map[string]string{"resourceConfigApiKey": "rcapikey456", "bucket": "mybucket"},
			expected: map[string]string{"resourceConfigApiKey": "rc*********56", "bucket": "mybucket"},
		},
		{
			name:     "with quotaLimit",
			input:    map[string]string{"quotaLimit": "true", "bucket": "mybucket"},
			expected: map[string]string{"quotaLimit": "****", "bucket": "mybucket"},
		},
		{
			name:     "with token",
			input:    map[string]string{"token": "mytoken789", "bucket": "mybucket"},
			expected: map[string]string{"token": "my*********89", "bucket": "mybucket"},
		},
		{
			name:     "with password",
			input:    map[string]string{"password": "mypass123", "bucket": "mybucket"},
			expected: map[string]string{"password": "ma*********23", "bucket": "mybucket"},
		},
		{
			name:     "with credential",
			input:    map[string]string{"credential": "cred456", "bucket": "mybucket"},
			expected: map[string]string{"credential": "cr*********56", "bucket": "mybucket"},
		},
		{
			name:     "mixed sensitive and non-sensitive",
			input:    map[string]string{"bucket": "mybucket", "accessKey": "key123", "region": "us-south", "apiKey": "apikey456"},
			expected: map[string]string{"bucket": "mybucket", "accessKey": "ke*********23", "region": "us-south", "apiKey": "ap*********56"},
		},
		{
			name:     "case insensitive key matching",
			input:    map[string]string{"AccessKey": "key123", "SECRETKEY": "secret456", "bucket": "mybucket"},
			expected: map[string]string{"AccessKey": "ke*********23", "SECRETKEY": "se*********56", "bucket": "mybucket"},
		},
		{
			name:     "bucketName and objectPath",
			input:    map[string]string{"bucketName": "mybucket", "objectPath": "path/to/object", "region": "us-south"},
			expected: map[string]string{"bucketName": "my*********et", "objectPath": "pa*********ct", "region": "us-south"},
		},
		{
			name:     "kpRootKeyCRN and serviceId",
			input:    map[string]string{"kpRootKeyCRN": "crn:v1:bluemix:public:kms:us-south:a/12345:key:abc", "serviceId": "service-123", "region": "us-south"},
			expected: map[string]string{"kpRootKeyCRN": "cr***********************************bc", "serviceId": "se*******23", "region": "us-south"},
		},
		{
			name:     "iamEndpoint and cosEndpoint",
			input:    map[string]string{"iamEndpoint": "https://iam.cloud.ibm.com", "cosEndpoint": "https://s3.us-south.cloud-object-storage.appdomain.cloud", "region": "us-south"},
			expected: map[string]string{"iamEndpoint": "ht*********om", "cosEndpoint": "ht*********ud", "region": "us-south"},
		},
		{
			name:     "locationConstraint",
			input:    map[string]string{"locationConstraint": "us-south", "region": "us-south"},
			expected: map[string]string{"locationConstraint": "us*****th", "region": "us-south"},
		},
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskParams(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
