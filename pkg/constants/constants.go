package constants

import "time"

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

	// Timeout specifies a time limit for requests made by HTTP Client
	Timeout                      = 3 * time.Minute
	COSCSIMounterSocketPath      = "/var/lib/coscsi-sock/coscsi.sock"
	COSCSIMounterSocketPathEnv   = "COS_CSI_MOUNTER_SOCKET"
	MounterConfigPathOnHost      = "/var/lib/coscsi-config"
	MounterConfigPathOnPodS3fs   = "/var/lib/ibmc-s3fs"
	MounterConfigPathOnPodRclone = "/root/.config/rclone"

	PVCNameKey         = "csi.storage.k8s.io/pvc/name"
	PVCNamespaceKey    = "csi.storage.k8s.io/pvc/namespace"
	SecretNameKey      = "cos.csi.driver/secret"           // #nosec G101 -- false positive, this is not a credential
	SecretNamespaceKey = "cos.csi.driver/secret-namespace" // #nosec G101 -- false positive, this is not a credential

	BucketVersioning = "bucketVersioning"

	IsNodeServer         = "IS_NODE_SERVER"
	KubeNodeName         = "KUBE_NODE_NAME"
	MaxVolumesPerNodeEnv = "MAX_VOLUMES_PER_NODE"
)

var (
	SocketDir  = "/var/lib/coscsi-sock"
	SocketFile = "coscsi.sock"
)
