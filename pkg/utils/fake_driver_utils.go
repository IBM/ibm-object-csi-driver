package utils

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ StatsUtils = (*FakeStatsUtilsFuncStructImpl)(nil)

type FakeStatsUtilsFuncStruct struct {
	FSInfoFn         func(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMountFn     func(targetPath string) (bool, error)
	BucketToDeleteFn func(volumeID string) (string, error)
	GetBucketUsageFn func(volumeID string) (int64, resource.Quantity, error)
}

type FakeStatsUtilsFuncStructImpl struct {
	DriverStatsUtils

	FuncStruct FakeStatsUtilsFuncStruct
}

func NewFakeStatsUtilsImpl(reqFn FakeStatsUtilsFuncStruct) *FakeStatsUtilsFuncStructImpl {
	return &FakeStatsUtilsFuncStructImpl{
		FuncStruct: reqFn,
	}
}

func (m *FakeStatsUtilsFuncStructImpl) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	if m.FuncStruct.FSInfoFn != nil {
		return m.FuncStruct.FSInfoFn(path)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) CheckMount(targetPath string) (bool, error) {
	if m.FuncStruct.CheckMountFn != nil {
		return m.FuncStruct.CheckMountFn(targetPath)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) BucketToDelete(volumeID string) (string, error) {
	if m.FuncStruct.BucketToDeleteFn != nil {
		return m.FuncStruct.BucketToDeleteFn(volumeID)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetBucketUsage(volumeID string) (int64, resource.Quantity, error) {
	if m.FuncStruct.GetBucketUsageFn != nil {
		return m.FuncStruct.GetBucketUsageFn(volumeID)
	}
	panic("requested method should not be nil")
}
