package mounter

import "github.com/IBM/ibm-object-csi-driver/pkg/constants"

const (
	bucketName = "fakeBucketName"
	objPath    = "fakePath"
	endPoint   = "fakeEndPoint"
	region     = "fakeRegion"
	keys       = "fakeKeys"
	authType   = "fakeAuthType"
)

type FakeMounterFactory struct {
	Mounter         string
	IsFailedMount   bool
	IsFailedUnmount bool
}

func (f *FakeMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) Mounter {
	switch f.Mounter {
	case constants.S3FS:
		return fakenewS3fsMounter(f.IsFailedMount, f.IsFailedUnmount)
	case constants.RClone:
		return fakenewRcloneMounter(f.IsFailedMount, f.IsFailedUnmount)
	default:
		return fakenewS3fsMounter(f.IsFailedMount, f.IsFailedUnmount)
	}
}
