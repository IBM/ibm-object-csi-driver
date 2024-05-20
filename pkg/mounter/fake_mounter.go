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
	Mounter string
}

func (f *FakeMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) (Mounter, error) {
	switch f.Mounter {
	case constants.S3FS:
		return fakenewS3fsMounter()
	case constants.RClone:
		return fakenewRcloneMounter()
	default:
		return fakenewS3fsMounter()
	}
}
