package utils

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type FakeStatsUtilsFuncStruct struct {
	FSInfoFn                 func(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMountFn             func(targetPath string) error
	BucketToDeleteFn         func(volumeID string) (string, error)
	GetTotalCapacityFromPVFn func(volumeID string) (resource.Quantity, error)
	GetBucketUsageFn         func(volumeID string) (int64, error)
	GetBucketNameFromPVFn    func(volumeID string) (string, error)
	GetRegionAndZoneFn       func(nodeName string) (string, string, error)
	GetPVAttributesFn        func(volumeID string) (map[string]string, error)
	GetPVCFn                 func(pvcName, pvcNamespace string) (*v1.PersistentVolumeClaim, error)
	GetSecretFn              func(secretName, secretNamespace string) (*v1.Secret, error)
	GetPVFn                  func(volumeID string) (*v1.PersistentVolume, error)
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

func (m *FakeStatsUtilsFuncStructImpl) CheckMount(targetPath string) error {
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

func (m *FakeStatsUtilsFuncStructImpl) GetTotalCapacityFromPV(volumeID string) (resource.Quantity, error) {
	if m.FuncStruct.GetTotalCapacityFromPVFn != nil {
		return m.FuncStruct.GetTotalCapacityFromPVFn(volumeID)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetBucketUsage(volumeID string) (int64, error) {
	if m.FuncStruct.GetBucketUsageFn != nil {
		return m.FuncStruct.GetBucketUsageFn(volumeID)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetBucketNameFromPV(volumeID string) (string, error) {
	if m.FuncStruct.GetBucketNameFromPVFn != nil {
		return m.FuncStruct.GetBucketNameFromPVFn(volumeID)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetRegionAndZone(nodeName string) (string, string, error) {
	if m.FuncStruct.GetRegionAndZoneFn != nil {
		return m.FuncStruct.GetRegionAndZoneFn(nodeName)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetPVAttributes(volumeID string) (map[string]string, error) {
	if m.FuncStruct.GetPVAttributesFn != nil {
		return m.FuncStruct.GetPVAttributesFn(volumeID)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetPVC(pvcName, pvcNamespace string) (*v1.PersistentVolumeClaim, error) {
	if m.FuncStruct.GetPVCFn != nil {
		return m.FuncStruct.GetPVCFn(pvcName, pvcNamespace)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetSecret(secretName, secretNamespace string) (*v1.Secret, error) {
	if m.FuncStruct.GetSecretFn != nil {
		return m.FuncStruct.GetSecretFn(secretName, secretNamespace)
	}
	panic("requested method should not be nil")
}

func (m *FakeStatsUtilsFuncStructImpl) GetPV(volumeID string) (*v1.PersistentVolume, error) {
	if m.FuncStruct.GetPVFn != nil {
		return m.FuncStruct.GetPVFn(volumeID)
	}
	panic("requested method should not be nil")
}
