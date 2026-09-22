package main

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
			name:     "path",
			input:    "/mnt/data",
			expected: "/m******ta",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskSensitive(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMaskArgs_Slice(t *testing.T) {
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

func TestMaskArgs_Map(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "no sensitive keys",
			input:    map[string]string{"endpoint": "s3.example.com", "path": "/mnt/data"},
			expected: map[string]string{"endpoint": "s3.example.com", "path": "/mnt/data"},
		},
		{
			name:     "with access key",
			input:    map[string]string{"accessKey": "mykey123", "endpoint": "s3.example.com"},
			expected: map[string]string{"accessKey": "my*********23", "endpoint": "s3.example.com"},
		},
		{
			name:     "with secret key",
			input:    map[string]string{"secretKey": "secret123", "endpoint": "s3.example.com"},
			expected: map[string]string{"secretKey": "se*********23", "endpoint": "s3.example.com"},
		},
		{
			name:     "with passwd",
			input:    map[string]string{"passwd_file": "/etc/passwd", "endpoint": "s3.example.com"},
			expected: map[string]string{"passwd_file": "pa*********wd", "endpoint": "s3.example.com"},
		},
		{
			name:     "multiple sensitive keys",
			input:    map[string]string{"accessKey": "key123", "secretKey": "secret456", "endpoint": "s3.example.com"},
			expected: map[string]string{"accessKey": "ke*********23", "secretKey": "se*********56", "endpoint": "s3.example.com"},
		},
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maskArgs(tc.input)
			resultMap, ok := result.(map[string]string)
			assert.True(t, ok)
			assert.Equal(t, tc.expected, resultMap)
		})
	}
}

func TestMaskArgs_UnknownType(t *testing.T) {
	input := 123
	result := maskArgs(input)
	assert.Equal(t, input, result)
}
