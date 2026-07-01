//go:build !linux
// +build !linux

package utils

import (
	"context"
	"fmt"
)

type MounterUtils interface {
	FuseUnmount(ctx context.Context, path string) error
	FuseMount(ctx context.Context, path string, comm string, args []string) error
}

type MounterOptsUtils struct{}

func (su *MounterOptsUtils) FuseMount(ctx context.Context, path string, comm string, args []string) error {
	return fmt.Errorf("FuseMount not supported on this platform")
}

func (su *MounterOptsUtils) FuseUnmount(ctx context.Context, path string) error {
	return fmt.Errorf("FuseUnmount not supported on this platform")
}
