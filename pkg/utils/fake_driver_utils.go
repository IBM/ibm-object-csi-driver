package utils

var _ StatsUtils = (*FakeStatsUtilsFuncStructImpl)(nil)

type FakeStatsUtilsFuncStruct struct {
	FuseMountFn      func(path string, comm string, args []string) error
	FSInfoFn         func(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMountFn     func(targetPath string) (bool, error)
	FuseUnmountFn    func(path string) error
	BucketToDeleteFn func(volumeID string) (string, error)
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
