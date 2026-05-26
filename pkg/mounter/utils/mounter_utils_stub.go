//go:build !linux
// +build !linux

package utils

import "fmt"

type MounterUtils interface {
	FuseUnmount(path string) error
	FuseMount(path string, comm string, args []string) error
}

type MounterOptsUtils struct{}

func (su *MounterOptsUtils) FuseMount(path string, comm string, args []string) error {
	return fmt.Errorf("FuseMount not supported on this platform")
}

func (su *MounterOptsUtils) FuseUnmount(path string) error {
	return fmt.Errorf("FuseUnmount not supported on this platform")
}
