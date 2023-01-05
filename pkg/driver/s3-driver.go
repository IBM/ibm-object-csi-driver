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
//Package driver ...
package driver

import (
	"fmt"

	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
)

const (
	kib    int64 = 1024
	mib    int64 = kib * 1024
	gib    int64 = mib * 1024
	gib10  int64 = gib * 10
	gib100 int64 = gib * 100
	tib    int64 = gib * 1024
	tib100 int64 = tib * 100
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

	cap   []*csi.ControllerServiceCapability
	vc    []*csi.VolumeCapability_AccessMode
	nscap []*csi.NodeServiceCapability
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

func newControllerServer(d *S3Driver, s3cosSession s3client.ObjectStorageSessionFactory) *controllerServer {
	return &controllerServer{
		S3Driver: d,
		cosSession: s3cosSession,
	}
}

func newNodeServer(d *S3Driver, nodeID string) *nodeServer {
	return &nodeServer{
		S3Driver: d,
		NodeID:   nodeID,
	}
}

func (driver *S3Driver) NewS3CosDriver(nodeID string, endpoint string, s3cosSession s3client.ObjectStorageSessionFactory) (*S3Driver, error) {
	s3client, err := s3client.NewS3Client(driver.logger)
	if err != nil {
		return nil, err
	}

	driver.endpoint = endpoint
	driver.s3client = s3client

	// Create GRPC servers
	driver.ids = newIdentityServer(driver)
	if driver.mode == "controller" {
		driver.cs = newControllerServer(driver, s3cosSession)
	}
	if driver.mode == "node" {
		driver.ns = newNodeServer(driver, nodeID)
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
