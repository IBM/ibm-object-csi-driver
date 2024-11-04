package constants

const (
	DefaultIAMEndPoint    = "https://iam.cloud.ibm.com"
	DefaultVolumesPerNode = 4

	KPEncryptionAlgorithm = "AES256" // https://github.com/IBM/ibm-cos-sdk-go/blob/master/service/s3/api.go#L9130-L9136

	S3FS             = "s3fs"
	RClone           = "rclone"
	DefaultNamespace = "default"

	IAMEP                   = "https://private.iam.cloud.ibm.com/identity/token"
	ResourceConfigEPPrivate = "https://config.private.cloud-object-storage.cloud.ibm.com/v1"
	ResourceConfigEPDirect  = "https://config.direct.cloud-object-storage.cloud.ibm.com/v1"
)
