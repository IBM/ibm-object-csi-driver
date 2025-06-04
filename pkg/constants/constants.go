package constants

const (
	DefaultIAMEndPoint = "https://iam.cloud.ibm.com"

	// Maximum number of volumes that controller can publish to the node.
	// If value is not set or zero CO SHALL decide how many volumes of
	// this type can be published by the controller to the node. The
	// plugin MUST NOT set negative values here.
	DefaultVolumesPerNode = 0

	KPEncryptionAlgorithm = "AES256" // https://github.com/IBM/ibm-cos-sdk-go/blob/master/service/s3/api.go#L9130-L9136

	S3FS             = "s3fs"
	RClone           = "rclone"
	DefaultNamespace = "default"

	IAMEP                   = "https://private.iam.cloud.ibm.com/identity/token"
	ResourceConfigEPPrivate = "https://config.private.cloud-object-storage.cloud.ibm.com/v1"
	ResourceConfigEPDirect  = "https://config.direct.cloud-object-storage.cloud.ibm.com/v1"

	// NodeZoneLabel  Zone Label attached to node
	NodeZoneLabel = "topology.kubernetes.io/zone"

	// NodeRegionLabel Region Label attached to node
	NodeRegionLabel = "topology.kubernetes.io/region"

	SocketDir  = "/var/lib/coscsi-sock"
	SocketFile = "coscsi.sock"
)
