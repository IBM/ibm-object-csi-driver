//go:build linux
// +build linux

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactMountArgs(t *testing.T) {
	args := []string{
		"balraj-cos",
		"/mount/path",
		"-o", "passwd_file=/var/lib/coscsi-config/abc/.passwd-s3fs",
		"-o", "allow_other",
		"-o", "ibm_api_key=super-secret",
		"-o", "secret_access_key=also-secret",
		"-o", "url=https://s3.example.com",
	}
	got := redactMountArgs(args)
	assert.Equal(t, "passwd_file=xxxxx", got[3])
	assert.Equal(t, "ibm_api_key=xxxxx", got[6])
	assert.Equal(t, "secret_access_key=xxxxx", got[8])
	// non-sensitive pass through unchanged
	assert.Equal(t, "allow_other", got[5])
	assert.Equal(t, "url=https://s3.example.com", got[9])
}
