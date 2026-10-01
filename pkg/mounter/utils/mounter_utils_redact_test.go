//go:build linux
// +build linux

package utils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactingWriter_MasksPasswdFile(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "s3fs startup banner with passwd_file",
			input: "s3fs version 1.94 : s3fs -o allow_other -o passwd_file=/var/lib/coscsi-config/abc123/.passwd-s3fs -o url=https://s3.example.com\n",
			want:  "s3fs version 1.94 : s3fs -o allow_other -o passwd_file=****** -o url=https://s3.example.com\n",
		},
		{
			name:  "line without passwd_file passes through unchanged",
			input: "bucket mounted successfully\n",
			want:  "bucket mounted successfully\n",
		},
		{
			name:  "passwd_file at end of line",
			input: "s3fs -o passwd_file=/some/path/.passwd-s3fs\n",
			want:  "s3fs -o passwd_file=******\n",
		},
		{
			name:  "multiple lines — only passwd_file line redacted",
			input: "starting s3fs\ns3fs -o passwd_file=/secret/path/.passwd-s3fs -o allow_other\nmount done\n",
			want:  "starting s3fs\ns3fs -o passwd_file=****** -o allow_other\nmount done\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			rw := &redactingWriter{w: &buf}
			_, err := rw.Write([]byte(tc.input))
			assert.NoError(t, err)
			rw.flush()
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func TestRedactingWriter_IncompleteLineBuffered(t *testing.T) {
	var buf bytes.Buffer
	rw := &redactingWriter{w: &buf}

	_, err := rw.Write([]byte("s3fs -o passwd_file=/secret/.passwd-s3fs -o url=https://s3.example.com"))
	assert.NoError(t, err)
	assert.Empty(t, buf.String(), "incomplete line must be buffered, not written yet")

	rw.flush()
	assert.Equal(t, "s3fs -o passwd_file=****** -o url=https://s3.example.com", buf.String())
}

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
