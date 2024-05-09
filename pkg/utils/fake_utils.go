package utils

var _ StatsUtils = (*MockStatsUtilsFuncStructImpl)(nil)

type MockStatsUtilsFuncStruct struct {
	BucketToDeleteFn func(volumeID string) (string, error)
	FSInfoFn         func(path string) (int64, int64, int64, int64, int64, int64, error)
	CheckMountFn     func(targetPath string) (bool, error)
	FuseUnmountFn    func(path string) error
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

func (m *MockStatsUtilsFuncStructImpl) BucketToDelete(volumeID string) (string, error) {
	if m.FuncStruct.BucketToDeleteFn != nil {
		return m.FuncStruct.BucketToDeleteFn(volumeID)
	}
	panic("requested method should not be nil")
}

func (m *MockStatsUtilsFuncStructImpl) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	panic("requested method should not be nil")
}

func (m *MockStatsUtilsFuncStructImpl) CheckMount(targetPath string) (bool, error) {
	panic("requested method should not be nil")
}

func (m *MockStatsUtilsFuncStructImpl) FuseUnmount(path string) error {
	panic("requested method should not be nil")
}
