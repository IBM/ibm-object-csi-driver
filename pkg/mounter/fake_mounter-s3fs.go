package mounter

import "errors"

type fakes3fsMounter struct {
	bucketName    string
	objectPath    string
	endPoint      string
	locConstraint string
	authType      string
	accessKeys    string
	kpRootKeyCrn  string

	isFailedMount   bool
	isFailedUnmount bool
}

func fakenewS3fsMounter(isFailedMount, isFailedUnmount bool) Mounter {
	return &fakes3fsMounter{
		bucketName:      bucketName,
		objectPath:      objectPath,
		endPoint:        endPoint,
		locConstraint:   region,
		accessKeys:      keys,
		authType:        authType,
		kpRootKeyCrn:    "",
		isFailedMount:   isFailedMount,
		isFailedUnmount: isFailedUnmount,
	}
}

func (s3fs *fakes3fsMounter) Mount(source string, target string) error {
	if s3fs.isFailedMount {
		return errors.New("failed to mount s3fs")
	}
	return nil
}

func (s3fs *fakes3fsMounter) Unmount(target string) error {
	if s3fs.isFailedUnmount {
		return errors.New("failed to unmount s3fs")
	}
	return nil
}
