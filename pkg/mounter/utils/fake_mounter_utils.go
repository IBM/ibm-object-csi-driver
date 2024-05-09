package utils

var _ MounterUtils = (*FakeMounterUtilsFuncStructImpl)(nil)

type FakeMounterUtilsFuncStruct struct {
	FuseMountFn   func(path string, comm string, args []string) error
	FuseUnmountFn func(path string) error
}

type FakeMounterUtilsFuncStructImpl struct {
	MounterOptsUtils

	FuncStruct FakeMounterUtilsFuncStruct
}

func NewFakeMounterUtilsImpl(reqFn FakeMounterUtilsFuncStruct) *FakeMounterUtilsFuncStructImpl {
	return &FakeMounterUtilsFuncStructImpl{
		FuncStruct: reqFn,
	}
}

func (m *FakeMounterUtilsFuncStructImpl) FuseMount(path string, comm string, args []string) error {
	if m.FuncStruct.FuseMountFn != nil {
		return m.FuncStruct.FuseMountFn(path, comm, args)
	}
	panic("requested method should not be nil")
}

func (m *FakeMounterUtilsFuncStructImpl) FuseUnmount(path string) error {
	if m.FuncStruct.FuseUnmountFn != nil {
		return m.FuncStruct.FuseUnmountFn(path)
	}
	panic("requested method should not be nil")
}
