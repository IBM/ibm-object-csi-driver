//go:build !linux
// +build !linux

package mounter

import (
	"context"
	"fmt"
)

type CSIMounterFactory struct{}

type NewMounterFactory interface {
	NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string, defaultMOMap map[string]string) Mounter
}

type Mounter interface {
	Mount(ctx context.Context, source string, target string) error
	Unmount(ctx context.Context, target string) error
}

func NewCSIMounterFactory() *CSIMounterFactory {
	return &CSIMounterFactory{}
}

func (s *CSIMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string, defaultMOMap map[string]string) Mounter {
	return &stubMounter{}
}

type stubMounter struct{}

func (s *stubMounter) Mount(ctx context.Context, source string, target string) error {
	return fmt.Errorf("mounter not supported on this platform")
}

func (s *stubMounter) Unmount(ctx context.Context, target string) error {
	return fmt.Errorf("mounter not supported on this platform")
}
