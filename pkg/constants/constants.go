package constants

const (
	kib int64 = 1024
	mib int64 = kib * 1024
	gib int64 = mib * 1024

	MaxStorageCapacity    = gib
	DefaultIAMEndPoint    = "https://iam.cloud.ibm.com"
	DefaultVolumesPerNode = 4
)
