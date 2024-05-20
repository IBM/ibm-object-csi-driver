package mounter

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
}

func fakenewRcloneMounter() Mounter {
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
	}
}

func (s3fs *fakercloneMounter) Mount(source string, target string) error {
	return nil
}

func (s3fs *fakercloneMounter) Unmount(target string) error {
	return nil
}
