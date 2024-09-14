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
	Mounter       string
	IsFailedMount bool
}

func (f *FakeMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) Mounter {
	switch f.Mounter {
	case constants.S3FS:
		return fakenewS3fsMounter(f.IsFailedMount)
	case constants.RClone:
		return fakenewRcloneMounter(f.IsFailedMount)
	case constants.MNTS3:
		return fakenewMountpointMounter(f.IsFailedMount)
	default:
		return fakenewS3fsMounter(f.IsFailedMount)
	}
}
