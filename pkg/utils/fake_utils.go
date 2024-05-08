package utils

import (
	"k8s.io/klog/v2"
)

var _ StatsUtils = (*MockStatsUtilsFuncStructImpl)(nil)

type MockStatsUtilsFuncStruct struct {
	FuseMountFn      func(path string, comm string, args []string) error
	FSInfoFn         func(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMountFn     func(targetPath string) (bool, error)
	FuseUnmountFn    func(path string) error
	BucketToDeleteFn func(volumeID string) (string, error)
}

type MockStatsUtilsFuncStructImpl struct {
	DriverStatsUtils

	FuncStruct MockStatsUtilsFuncStruct
}

func NewMockStatsUtilsImpl(reqFn MockStatsUtilsFuncStruct) *MockStatsUtilsFuncStructImpl {
	return &MockStatsUtilsFuncStructImpl{
		FuncStruct: reqFn,
	}
}

func (m *MockStatsUtilsFuncStructImpl) FuseMount(path string, comm string, args []string) error {
	if m.FuncStruct.FuseMountFn != nil {
		return m.FuncStruct.FuseMountFn(path, comm, args)
	}
	panic("requested method should not be nil")
}

func (m *MockStatsUtilsFuncStructImpl) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	if m.FuncStruct.FSInfoFn != nil {
		return m.FuncStruct.FSInfoFn(path)
	}
	panic("requested method should not be nil")
}

func (m *MockStatsUtilsFuncStructImpl) CheckMount(targetPath string) (bool, error) {
	if m.FuncStruct.CheckMountFn != nil {
		return m.FuncStruct.CheckMountFn(targetPath)
	}
	panic("requested method should not be nil")
}

func (m *MockStatsUtilsFuncStructImpl) FuseUnmount(path string) error {
	klog.Info("-fake-fuseUnmount-")
	if m.FuncStruct.FuseUnmountFn != nil {
		return m.FuncStruct.FuseUnmountFn(path)
	}
	panic("requested method should not be nil")
}

func (m *MockStatsUtilsFuncStructImpl) BucketToDelete(volumeID string) (string, error) {
	if m.FuncStruct.BucketToDeleteFn != nil {
		return m.FuncStruct.BucketToDeleteFn(volumeID)
	}
	panic("requested method should not be nil")
}
