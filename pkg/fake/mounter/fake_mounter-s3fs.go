package mounter

import "github.com/IBM/ibm-object-csi-driver/pkg/mounter"

type fakes3fsMounter struct {
	bucketName    string
	objPath       string
	endPoint      string
	locConstraint string
	authType      string
	accessKeys    string
	kpRootKeyCrn  string
}

func fakenewS3fsMounter() (mounter.Mounter, error) {
	return &fakes3fsMounter{
		bucketName:    bucketName,
		objPath:       objPath,
		endPoint:      endPoint,
		locConstraint: region,
		accessKeys:    keys,
		authType:      authType,
		kpRootKeyCrn:  "",
	}, nil
}

func (s3fs *fakes3fsMounter) Mount(source string, target string) error {
	return nil
}

func (s3fs *fakes3fsMounter) Unmount(target string) error {
	return nil
}
