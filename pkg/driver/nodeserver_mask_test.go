package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeserverMaskSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "long bucket name",
			input:    "my-bucket-name-12345",
			expected: "my*********45",
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
			name:     "error message with sensitive data",
			input:    "failed to mount bucket mybucket with key abc123",
			expected: "fa*********23",
		},
		{
			name:     "endpoint URL",
			input:    "https://s3.us-south.cloud-object-storage.appdomain.cloud",
			expected: "ht*********ud",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskSensitive(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
