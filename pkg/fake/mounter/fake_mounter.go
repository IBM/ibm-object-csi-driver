package mounter

import (
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
)

const (
	bucketName = "fakeBucketName"
	objPath    = "fakePath"
	endPoint   = "fakeEndPoint"
	region     = "fakeRegion"
	keys       = "fakeKeys"
	authType   = "fakeAuthType"
)

func NewMounter(mounter string) (mounter.Mounter, error) {
	switch mounter {
	case constants.S3FS:
		return fakenewS3fsMounter()
	case constants.RClone:
		return fakenewRcloneMounter()
	default:
		// default to s3backer
		return fakenewS3fsMounter()
	}
}
