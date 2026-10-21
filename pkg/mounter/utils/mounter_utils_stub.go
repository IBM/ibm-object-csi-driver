//go:build !linux

package utils

// MounterUtils defines the interface for FUSE mount/unmount operations.
// On Linux the real implementation lives in mounter_utils.go.
// This stub allows the package to compile and be tested on non-Linux hosts.
type MounterUtils interface {
	FuseUnmount(path string) error
	FuseMount(path string, comm string, args []string) error
}

// MounterOptsUtils is an empty struct whose methods are defined in mounter_utils.go (Linux only).
// Declared here so the rest of the package compiles on non-Linux.
type MounterOptsUtils struct{}

func (su *MounterOptsUtils) FuseMount(_ string, _ string, _ []string) error { return nil }
func (su *MounterOptsUtils) FuseUnmount(_ string) error                      { return nil }
