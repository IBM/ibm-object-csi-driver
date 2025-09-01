/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

import (
	"fmt"
	"os"
	"strconv"

	"github.com/IBM/ibm-csi-common/pkg/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	pkgUtils "github.com/IBM/ibm-object-csi-driver/pkg/utils"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"k8s.io/klog/v2"
)

var (
	// volumeCapabilities represents how the volume could be accessed.
	volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
	}

	// controllerCapabilities represents the capability of controller service
	controllerCapabilities = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	}

	// nodeServerCapabilities represents the capability of node service.
	nodeServerCapabilities = []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
		csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
	}
)

type S3Driver struct {
	name        string
	version     string
	mode        string
	endpoint    string
	iamEndpoint string

	s3client s3client.ObjectStorageSession

	ids *identityServer
	ns  *nodeServer
	cs  *controllerServer

	logger *zap.Logger
	vcap   []*csi.VolumeCapability_AccessMode
	cscap  []*csi.ControllerServiceCapability
	nscap  []*csi.NodeServiceCapability
}

// AddVolumeCapabilityAccessModes ...
func (driver *S3Driver) AddVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) error {
	driver.logger.Info("IBMCSIDriver-AddVolumeCapabilityAccessModes...", zap.Reflect("VolumeCapabilityAccessModes", vc))
	var vca []*csi.VolumeCapability_AccessMode
	for _, c := range vc {
		driver.logger.Info("Enabling volume access mode", zap.Reflect("Mode", c.String()))
		vca = append(vca, utils.NewVolumeCapabilityAccessMode(c))
	}
	driver.vcap = vca
	driver.logger.Info("Successfully enabled Volume Capability Access Modes")
	return nil
}

// AddControllerServiceCapabilities ...
func (driver *S3Driver) AddControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) error {
	driver.logger.Info("IBMCSIDriver-AddControllerServiceCapabilities...", zap.Reflect("ControllerServiceCapabilities", cl))
	var csc []*csi.ControllerServiceCapability
	for _, c := range cl {
		driver.logger.Info("Adding controller service capability", zap.Reflect("Capability", c.String()))
		csc = append(csc, utils.NewControllerServiceCapability(c))
	}
	driver.cscap = csc
	driver.logger.Info("Successfully added Controller Service Capabilities")
	return nil
}

// AddNodeServiceCapabilities ...
func (driver *S3Driver) AddNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) error {
	driver.logger.Info("IBMCSIDriver-AddNodeServiceCapabilities...", zap.Reflect("NodeServiceCapabilities", nl))
	var nsc []*csi.NodeServiceCapability
	for _, n := range nl {
		driver.logger.Info("Adding node service capability", zap.Reflect("NodeServiceCapabilities", n.String()))
		nsc = append(nsc, utils.NewNodeServiceCapability(n))
	}
	driver.nscap = nsc
	driver.logger.Info("Successfully added Node Service Capabilities")
	return nil
}

func Setups3Driver(mode, name, version string, lgr *zap.Logger) (*S3Driver, error) {
	csiDriver := &S3Driver{}
	csiDriver.logger = lgr
	csiDriver.logger.Info("S3CSIDriver-SetupS3CSIDriver setting up S3 CSI Driver")
	if name == "" {
		return nil, fmt.Errorf("driver name missing")
	}
	csiDriver.logger.Info("Driver Name", zap.String("name", name), zap.String("mode", mode))
	csiDriver.name = name
	csiDriver.version = version
	csiDriver.mode = mode

	csiDriver.logger.Info("successfully setup CSI driver")
	return csiDriver, nil
}

func newIdentityServer(d *S3Driver) *identityServer {
	return &identityServer{
		S3Driver: d,
	}
}

func newControllerServer(d *S3Driver, statsUtil pkgUtils.StatsUtils, s3cosSession s3client.ObjectStorageSessionFactory, logger *zap.Logger) *controllerServer {
	return &controllerServer{
		S3Driver:   d,
		Stats:      statsUtil,
		cosSession: s3cosSession,
		Logger:     logger,
	}
}

func newNodeServer(d *S3Driver, statsUtil pkgUtils.StatsUtils, nodeID string, mountObj mounter.NewMounterFactory, mounterUtil mounterUtils.MounterUtils) (*nodeServer, error) {
	nodeName := os.Getenv(constants.KubeNodeName)
	if nodeName == "" {
		return nil, fmt.Errorf("KUBE_NODE_NAME env variable not set")
	}

	data, err := statsUtil.GetNodeServerData(nodeName)
	if err != nil {
		return nil, err
	}

	var maxVolumesPerNode int64
	maxVolumesPerNodeStr := os.Getenv(constants.MaxVolumesPerNodeEnv)
	if maxVolumesPerNodeStr != "" {
		maxVolumesPerNode, err = strconv.ParseInt(maxVolumesPerNodeStr, 10, 64)
		if err != nil {
			return nil, err
		}
	} else {
		d.logger.Warn("MAX_VOLUMES_PER_NODE env variable not set. Using default value")
		maxVolumesPerNode = int64(constants.DefaultVolumesPerNode)
	}

	return &nodeServer{
		S3Driver: d,
		Stats:    statsUtil,
		NodeServerConfig: NodeServerConfig{MaxVolumesPerNode: maxVolumesPerNode, Region: data.Region, Zone: data.Zone,
			NodeID: nodeID, CipherSuites: data.CipherSuites},
		Mounter:      mountObj,
		MounterUtils: mounterUtil,
	}, nil
}

func (driver *S3Driver) NewS3CosDriver(nodeID string, endpoint string, s3cosSession s3client.ObjectStorageSessionFactory, mountObj mounter.NewMounterFactory, statsUtil pkgUtils.StatsUtils, mounterUtil mounterUtils.MounterUtils) (*S3Driver, error) {
	s3client, err := s3client.NewS3Client(driver.logger)
	if err != nil {
		return nil, err
	}

	iamEP, _, err := statsUtil.GetEndpoints()
	if err != nil {
		return nil, err
	}
	klog.Infof("iam endpoint: %v", iamEP)
	driver.iamEndpoint = iamEP

	driver.endpoint = endpoint
	driver.s3client = s3client

	_ = driver.AddVolumeCapabilityAccessModes(volumeCapabilities)       // #nosec G104: Attempt to AddVolumeCapabilityAccessModes only on best-effort basis.Error cannot be usefully handled.
	_ = driver.AddControllerServiceCapabilities(controllerCapabilities) // #nosec G104: Attempt to AddControllerServiceCapabilities only on best-effort basis.Error cannot be usefully handled.
	_ = driver.AddNodeServiceCapabilities(nodeServerCapabilities)       // #nosec G104: Attempt to AddNodeServiceCapabilities only on best-effort basis.Error cannot be usefully handled.

	// Create GRPC servers
	driver.ids = newIdentityServer(driver)
	switch driver.mode {
	case "controller":
		driver.cs = newControllerServer(driver, statsUtil, s3cosSession, driver.logger)
	case "node":
		driver.ns, err = newNodeServer(driver, statsUtil, nodeID, mountObj, mounterUtil)
	case "controller-node":
		driver.cs = newControllerServer(driver, statsUtil, s3cosSession, driver.logger)
		driver.ns, err = newNodeServer(driver, statsUtil, nodeID, mountObj, mounterUtil)
	}

	return driver, err
}

func (driver *S3Driver) Run() {
	driver.logger.Info("--S3CSIDriver Run--")
	driver.logger.Info("Driver:", zap.Reflect("Driver Name", driver.name))
	driver.logger.Info("Version:", zap.Reflect("Driver Version", driver.version))
	// Initialize default library driver

	grpcServer := NewNonBlockingGRPCServer(driver.mode, driver.logger)
	grpcServer.Start(driver.endpoint, driver.ids, driver.cs, driver.ns)
	grpcServer.Wait()
}
