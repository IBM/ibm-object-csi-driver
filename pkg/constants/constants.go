package constants

const (
	kib int64 = 1024
	mib int64 = kib * 1024
	gib int64 = mib * 1024

	MaxStorageCapacity    = gib
	DefaultIAMEndPoint    = "https://iam.cloud.ibm.com"
	DefaultVolumesPerNode = 4

	KPEncryptionAlgorithm = "AES256" // https://github.com/IBM/ibm-cos-sdk-go/blob/master/service/s3/api.go#L9130-L9136

	S3FS   = "s3fs"
	RClone = "rclone"
)
