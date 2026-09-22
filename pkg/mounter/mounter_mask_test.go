package mounter

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
			name:     "object path",
			input:    "path/to/object",
			expected: "pa*********ct",
		},
		{
			name:     "add mount param",
			input:    "option1=value1,option2=value2",
			expected: "op*********e2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskSensitive(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMaskArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no sensitive args",
			input:    []string{"mount", "/path", "bucket"},
			expected: []string{"mount", "/path", "bucket"},
		},
		{
			name:     "with passwd_file",
			input:    []string{"-o", "passwd_file=/etc/passwd", "-o", "url=https://example.com"},
			expected: []string{"-o", "pa*********wd", "-o", "ur*********om"},
		},
		{
			name:     "with access key in arg",
			input:    []string{"-o", "accessKey=mykey123"},
			expected: []string{"-o", "ac*********23"},
		},
		{
			name:     "with secret key in arg",
			input:    []string{"-o", "secretKey=secret123"},
			expected: []string{"-o", "se*********23"},
		},
		{
			name:     "with token in arg",
			input:    []string{"-o", "token=mytoken456"},
			expected: []string{"-o", "my*********56"},
		},
		{
			name:     "with password in arg",
			input:    []string{"-o", "password=mypass789"},
			expected: []string{"-o", "ma*********89"},
		},
		{
			name:     "with credential in arg",
			input:    []string{"-o", "credential=cred123"},
			expected: []string{"-o", "cr*********23"},
		},
		{
			name:     "mixed sensitive and non-sensitive",
			input:    []string{"mount", "-o", "url=https://example.com", "-o", "accessKey=mykey123", "-o", "allow_other"},
			expected: []string{"mount", "-o", "ur*********om", "-o", "ac*********23", "-o", "allow_other"},
		},
		{
			name:     "case insensitive matching",
			input:    []string{"-o", "ACCESSKEY=key123", "-o", "SecretKey=secret456"},
			expected: []string{"-o", "ke*********23", "-o", "se*********56"},
		},
		{
			name:     "short sensitive value",
			input:    []string{"-o", "key=abc"},
			expected: []string{"-o", "****"},
		},
		{
			name:     "empty args",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskArgs(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
