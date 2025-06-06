/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"os"
	"strconv"

	"go.uber.org/zap"
)

const (
	filePermission = 0660
)

//counterfeiter:generate . socketPermission

// socketPermission represents file system operations
type socketPermission interface {
	Chown(name string, uid, gid int) error
	Chmod(name string, mode os.FileMode) error
}

// realSocketPermission implements socketPermission
type opsSocketPermission struct{}

func (f *opsSocketPermission) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (f *opsSocketPermission) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

// setupSidecar updates owner/group and permission of the file given(addr)
func setupSidecar(addr string, ops socketPermission, logger *zap.Logger) error {
	groupSt := os.Getenv("SIDECAR_GROUP_ID")

	logger.Info("Setting owner and permissions of csi socket file. SIDECAR_GROUP_ID env must match the 'livenessprobe' sidecar container groupID for csi socket connection.")

	// If env is not set, set default to 0
	if groupSt == "" {
		logger.Warn("Unable to fetch SIDECAR_GROUP_ID environment variable. Sidecar container(s) might fail...")
		groupSt = "0"
	}

	group, err := strconv.Atoi(groupSt)
	if err != nil {
		return err
	}

	// Change group of csi socket to non-root user for enabling the csi sidecar
	if err := ops.Chown(addr, -1, group); err != nil {
		return err
	}

	// Modify permissions of csi socket
	// Only the users and the group owners will have read/write access to csi socket
	if err := ops.Chmod(addr, filePermission); err != nil {
		return err
	}

	logger.Info("Successfully set owner and permissions of csi socket file.")

	return nil
}
