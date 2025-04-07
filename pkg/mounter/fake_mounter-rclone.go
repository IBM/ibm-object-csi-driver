package mounter

import "errors"

type fakercloneMounter struct {
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

func fakenewRcloneMounter(isFailedMount bool) Mounter {
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

func (rclone *fakercloneMounter) Mount(source string, target string) error {
	if rclone.isFailedMount {
		return errors.New("failed to mount rclone")
	}
	return nil
}

func (rclone *fakercloneMounter) Unmount(target string) error {
	return nil
}
