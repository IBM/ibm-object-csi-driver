//go:build linux
// +build linux

package utils

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
			name:     "long string",
			input:    "my-api-key-12345",
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
			name:     "special characters",
			input:    "accessKey=abc123",
			expected: "ac*********23",
		},
		{
			name:     "api key with prefix",
			input:    "apiKey=xyz789",
			expected: "ap*****89",
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
			name:     "with access key",
			input:    []string{"-o", "passwd_file=/etc/passwd", "-o", "accessKey=mykey123"},
			expected: []string{"-o", "passwd_file=/etc/passwd", "-o", "ac*********23"},
		},
		{
			name:     "with secret key",
			input:    []string{"-o", "secretKey=secret123"},
			expected: []string{"-o", "se*********23"},
		},
		{
			name:     "with token",
			input:    []string{"-o", "token=mytoken456"},
			expected: []string{"-o", "my*********56"},
		},
		{
			name:     "with password",
			input:    []string{"-o", "password=mypass789"},
			expected: []string{"-o", "ma*********89"},
		},
		{
			name:     "with credential",
			input:    []string{"-o", "credential=cred123"},
			expected: []string{"-o", "cr*********23"},
		},
		{
			name:     "mixed sensitive and non-sensitive",
			input:    []string{"mount", "-o", "url=https://example.com", "-o", "accessKey=mykey123", "-o", "allow_other"},
			expected: []string{"mount", "-o", "url=https://example.com", "-o", "ac*********23", "-o", "allow_other"},
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
