package mounter

import "errors"

type fakes3fsMounter struct {
	bucketName    string
	objPath       string
	endPoint      string
	locConstraint string
	authType      string
	accessKeys    string
	kpRootKeyCrn  string

	isFailedMount bool
}

func fakenewS3fsMounter(isFailedMount bool) Mounter {
	return &fakes3fsMounter{
		bucketName:    bucketName,
		objPath:       objPath,
		endPoint:      endPoint,
		locConstraint: region,
		accessKeys:    keys,
		authType:      authType,
		kpRootKeyCrn:  "",
		isFailedMount: isFailedMount,
	}
}

func (s3fs *fakes3fsMounter) Mount(source string, target string) error {
	if s3fs.isFailedMount {
		return errors.New("failed to mount s3fs")
	}
	return nil
}

func (s3fs *fakes3fsMounter) Unmount(target string) error {
	return nil
}
