package mounter

import "errors"

type fakemountpointMounter struct {
	bucketName    string
	objPath       string
	endPoint      string
	accessKey     string
	secretKey     string
	isFailedMount bool
}

func fakenewMountpointMounter(isFailedMount bool) Mounter {
	return &fakemountpointMounter{
		bucketName:    bucketName,
		objPath:       objPath,
		endPoint:      endPoint,
		accessKey:     keys,
		secretKey:     keys,
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
