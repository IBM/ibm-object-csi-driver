package mounter

import "errors"

type fakercloneMounter struct {
	bucketName    string
	objectPath    string
	endPoint      string
	locConstraint string
	authType      string
	accessKeys    string
	kpRootKeyCrn  string
	uid           string
	gid           string

	isFailedMount   bool
	isFailedUnmount bool
}

func fakenewRcloneMounter(isFailedMount, isFailedUnmount bool) Mounter {
	return &fakercloneMounter{
		bucketName:      bucketName,
		objectPath:      objectPath,
		endPoint:        endPoint,
		locConstraint:   region,
		accessKeys:      keys,
		authType:        authType,
		kpRootKeyCrn:    "",
		uid:             "",
		gid:             "",
		isFailedMount:   isFailedMount,
		isFailedUnmount: isFailedUnmount,
	}
}

func (rclone *fakercloneMounter) Mount(source string, target string) error {
	if rclone.isFailedMount {
		return errors.New("failed to mount rclone")
	}
	return nil
}

func (rclone *fakercloneMounter) Unmount(target string) error {
	if rclone.isFailedUnmount {
		return errors.New("failed to unmount rclone")
	}
	return nil
}
