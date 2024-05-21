package constants

const (
	DefaultIAMEndPoint    = "https://iam.cloud.ibm.com"
	DefaultVolumesPerNode = 4

	KPEncryptionAlgorithm = "AES256" // https://github.com/IBM/ibm-cos-sdk-go/blob/master/service/s3/api.go#L9130-L9136

	S3FS   = "s3fs"
	RClone = "rclone"
)
