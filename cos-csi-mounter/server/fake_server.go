//go:build !unit_test
// +build !unit_test

package main

import (
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
)

// Mock implementations
type MockMounterUtils struct {
	mock.Mock
}

func (m *MockMounterUtils) FuseMount(path string, mounter string, args []string) error {
	argsCalled := m.Called(path, mounter, args)
	return argsCalled.Error(0)
}

func (m *MockMounterUtils) FuseUnmount(path string) error {
	argsCalled := m.Called(path)
	return argsCalled.Error(0)
}

type fakeListener struct{}

func (d *fakeListener) Accept() (net.Conn, error) {
	time.Sleep(100 * time.Millisecond)
	return nil, errors.New("simulated shutdown")
}
func (d *fakeListener) Close() error   { return nil }
func (d *fakeListener) Addr() net.Addr { return &net.UnixAddr{Name: "fake", Net: "unix"} }

func fakeSetupSocketSuccess() (net.Listener, error) {
	return &fakeListener{}, nil
}

func fakeRouter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

func fakeHandleSignals() {}

func fakeSetupSocketFail() (net.Listener, error) {
	return nil, errors.New("socket creation error")
}

type MockMounterArgsParser struct {
	mock.Mock
}

func (m *MockMounterArgsParser) Parse(request MountRequest) ([]string, error) {
	args := m.Called(request)
	return args.Get(0).([]string), args.Error(1)
}
