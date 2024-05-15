/**
 * Copyright 2021 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package driver

import (
	"fmt"

	"github.com/IBM/ibm-csi-common/pkg/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	pkgUtils "github.com/IBM/ibm-object-csi-driver/pkg/utils"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
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
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
		csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
	}
)

type S3Driver struct {
	name     string
	version  string
	mode     string
	endpoint string

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

func newNodeServer(d *S3Driver, statsUtil pkgUtils.StatsUtils, nodeID string, mountObj mounter.NewMounterFactory, mounterUtil mounterUtils.MounterUtils) *nodeServer {
	return &nodeServer{
		S3Driver:     d,
		Stats:        statsUtil,
		NodeID:       nodeID,
		Mounter:      mountObj,
		MounterUtils: mounterUtil,
	}
}

func (driver *S3Driver) NewS3CosDriver(nodeID string, endpoint string, s3cosSession s3client.ObjectStorageSessionFactory, mountObj mounter.NewMounterFactory, statsUtil pkgUtils.StatsUtils, mounterUtil mounterUtils.MounterUtils) (*S3Driver, error) {
	s3client, err := s3client.NewS3Client(driver.logger)
	if err != nil {
		return nil, err
	}

	driver.endpoint = endpoint
	driver.s3client = s3client

	_ = driver.AddVolumeCapabilityAccessModes(volumeCapabilities)       // #nosec G104: Attempt to AddVolumeCapabilityAccessModes only on best-effort basis.Error cannot be usefully handled.
	_ = driver.AddControllerServiceCapabilities(controllerCapabilities) // #nosec G104: Attempt to AddControllerServiceCapabilities only on best-effort basis.Error cannot be usefully handled.
	_ = driver.AddNodeServiceCapabilities(nodeServerCapabilities)       // #nosec G104: Attempt to AddNodeServiceCapabilities only on best-effort basis.Error cannot be usefully handled.

	// Create GRPC servers
	driver.ids = newIdentityServer(driver)
	if driver.mode == "controller" {
		driver.cs = newControllerServer(driver, statsUtil, s3cosSession, driver.logger)
	} else if driver.mode == "node" {
		driver.ns = newNodeServer(driver, statsUtil, nodeID, mountObj, mounterUtil)
	} else if driver.mode == "controller-node" {
		driver.cs = newControllerServer(driver, statsUtil, s3cosSession, driver.logger)
		driver.ns = newNodeServer(driver, statsUtil, nodeID, mountObj, mounterUtil)
	}

	return driver, nil
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
