package utils

var _ MounterUtils = (*MockMounterUtilsFuncStructImpl)(nil)

type MockMounterUtilsFuncStruct struct {
	FuseMountFn   func(path string, comm string, args []string) error
	FuseUnmountFn func(path string) error
}

type MockMounterUtilsFuncStructImpl struct {
	MounterOptsUtils

	FuncStruct MockMounterUtilsFuncStruct
}

func NewMockMounterUtilsImpl(reqFn MockMounterUtilsFuncStruct) *MockMounterUtilsFuncStructImpl {
	return &MockMounterUtilsFuncStructImpl{
		FuncStruct: reqFn,
	}
}

func (m *MockMounterUtilsFuncStructImpl) FuseMount(path string, comm string, args []string) error {
	if m.FuncStruct.FuseMountFn != nil {
		return m.FuncStruct.FuseMountFn(path, comm, args)
	}
	panic("requested method should not be nil")
}

func (m *MockMounterUtilsFuncStructImpl) FuseUnmount(path string) error {
	if m.FuncStruct.FuseUnmountFn != nil {
		return m.FuncStruct.FuseUnmountFn(path)
	}
	panic("requested method should not be nil")
}
