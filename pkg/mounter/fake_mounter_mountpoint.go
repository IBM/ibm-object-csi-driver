package mounter

import "errors"

type fakemountpointMounter struct {
	bucketName    string
	objPath       string
	endPoint      string
	locConstraint string
	authType      string
	accessKeys    string
	kpRootKeyCrn  string
	uid           string
	gid           string

	isFailedMount bool
}

func fakenewMountpointMounter(isFailedMount bool) Mounter {
	return &fakercloneMounter{
		bucketName:    bucketName,
		objPath:       objPath,
		endPoint:      endPoint,
		locConstraint: region,
		accessKeys:    keys,
		authType:      authType,
		kpRootKeyCrn:  "",
		uid:           "",
		gid:           "",
		isFailedMount: isFailedMount,
	}
}

func (mnpt *fakemountpointMounter) Mount(source string, target string) error {
	if mnpt.isFailedMount {
		return errors.New("failed to mount mountpoint")
	}
	return nil
}

func (mnpt *fakemountpointMounter) Unmount(target string) error {
	return nil
}
