/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2025 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockSocketPermission implements the socketPermission interface
type mockSocketPermission struct {
	chownCalled int
	chmodCalled int

	chownArgs struct {
		name string
		uid  int
		gid  int
	}
	chmodArgs struct {
		name string
		mode os.FileMode
	}

	chownErr error
	chmodErr error
}

func (m *mockSocketPermission) Chown(name string, uid, gid int) error {
	m.chownCalled++
	m.chownArgs = struct {
		name string
		uid  int
		gid  int
	}{name, uid, gid}
	return m.chownErr
}

func (m *mockSocketPermission) Chmod(name string, mode os.FileMode) error {
	m.chmodCalled++
	m.chmodArgs = struct {
		name string
		mode os.FileMode
	}{name, mode}
	return m.chmodErr
}

func TestSetupSidecar(t *testing.T) {
	tests := []struct {
		name               string
		groupID            string
		expectedErr        bool
		chownErr           error
		chmodErr           error
		expectedChownCalls int
		expectedChmodCalls int
		expectedGroupID    int
	}{
		{
			name:               "ValidGroupID",
			groupID:            "2121",
			expectedErr:        false,
			chownErr:           nil,
			chmodErr:           nil,
			expectedChownCalls: 1,
			expectedChmodCalls: 1,
			expectedGroupID:    2121,
		},
		{
			name:               "EmptyGroupID",
			groupID:            "",
			expectedErr:        false,
			chownErr:           nil,
			chmodErr:           nil,
			expectedChownCalls: 1,
			expectedChmodCalls: 1,
			expectedGroupID:    0, // Default to 0 if SIDECAR_GROUP_ID is empty
		},
		{
			name:               "ChownError",
			groupID:            "1000",
			expectedErr:        true,
			chownErr:           errors.New("chown error"),
			chmodErr:           nil,
			expectedChownCalls: 1,
			expectedChmodCalls: 0, // No chmod expected if chown fails
			expectedGroupID:    1000,
		},
		{
			name:               "ChmodError",
			groupID:            "1000",
			expectedErr:        true,
			chownErr:           nil,
			chmodErr:           errors.New("chmod error"),
			expectedChownCalls: 1,
			expectedChmodCalls: 1,
			expectedGroupID:    1000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set SIDECAR_GROUP_ID environment variable
			if tc.groupID != "" {
				err := os.Setenv("SIDECAR_GROUP_ID", tc.groupID)
				assert.NoError(t, err)
			} else {
				err := os.Unsetenv("SIDECAR_GROUP_ID")
				assert.NoError(t, err)
			}
			defer os.Unsetenv("SIDECAR_GROUP_ID") // nolint:errcheck

			mock := &mockSocketPermission{
				chownErr: tc.chownErr,
				chmodErr: tc.chmodErr,
			}

			// Creating test logger
			logger, teardown := GetTestLogger(t)
			defer teardown()

			// Call the function under test
			err := setupSidecar("/path/to/socket", mock, logger)

			// Verify the result
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify the number of times Chown and Chmod were called
			assert.Equal(t, tc.expectedChownCalls, mock.chownCalled)
			assert.Equal(t, tc.expectedChmodCalls, mock.chmodCalled)

			if tc.expectedChownCalls > 0 {
				assert.Equal(t, -1, mock.chownArgs.uid)
				assert.Equal(t, tc.expectedGroupID, mock.chownArgs.gid)
				assert.Equal(t, "/mock/socket", mock.chownArgs.name)
			}

			if tc.expectedChmodCalls > 0 {
				assert.Equal(t, filePermission, mock.chmodArgs.mode)
				assert.Equal(t, "/mock/socket", mock.chmodArgs.name)
			}
		})
	}
}
