package mounter

import (
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
)

const (
	bucketName = "fakeBucketName"
	objectPath = "fakePath"
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

func (f *FakeMounterFactory) NewMounter(params MounterParams) Mounter {
	switch f.Mounter {
	case constants.S3FS:
		return fakenewS3fsMounter(f.IsFailedMount, f.IsFailedUnmount)
	case constants.RClone:
		return fakenewRcloneMounter(f.IsFailedMount, f.IsFailedUnmount)
	default:
		return fakenewS3fsMounter(f.IsFailedMount, f.IsFailedUnmount)
	}
}
