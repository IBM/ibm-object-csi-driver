package utils

import "context"

type FakeMounterUtilsFuncStruct struct {
	FuseMountFn   func(ctx context.Context, path string, comm string, args []string) error
	FuseUnmountFn func(ctx context.Context, path string) error
}

type FakeMounterUtilsFuncStructImpl struct {
	FuncStruct FakeMounterUtilsFuncStruct
}

func NewFakeMounterUtilsImpl(reqFn FakeMounterUtilsFuncStruct) *FakeMounterUtilsFuncStructImpl {
	return &FakeMounterUtilsFuncStructImpl{
		FuncStruct: reqFn,
	}
}

func (m *FakeMounterUtilsFuncStructImpl) FuseMount(ctx context.Context, path string, comm string, args []string) error {
	if m.FuncStruct.FuseMountFn != nil {
		return m.FuncStruct.FuseMountFn(ctx, path, comm, args)
	}
	panic("requested method should not be nil")
}

func (m *FakeMounterUtilsFuncStructImpl) FuseUnmount(ctx context.Context, path string) error {
	if m.FuncStruct.FuseUnmountFn != nil {
		return m.FuncStruct.FuseUnmountFn(ctx, path)
	}
	panic("requested method should not be nil")
}
