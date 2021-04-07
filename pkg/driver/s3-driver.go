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

type s3Driver struct {
	name     string
	s3client s3client.ObjectStorageSession
	endpoint string

	ids           *identityServer
	ns            *nodeServer
	cs            *controllerServer
	vendorVersion string
	logger        *zap.Logger

	cap   []*csi.ControllerServiceCapability
	vc    []*csi.VolumeCapability_AccessMode
	nscap []*csi.NodeServiceCapability
}

func Setups3Driver(lgr *zap.Logger, name, vendorVersion string) (*s3Driver, error) {
	csiDriver := &s3Driver{}
	csiDriver.logger = lgr
	csiDriver.logger.Info("S3CSIDriver-SetupS3CSIDriver setting up S3 CSI Driver...")

	if name == "" {
		return nil, fmt.Errorf("driver name missing")
	}

	csiDriver.name = name
	csiDriver.vendorVersion = vendorVersion

	csiDriver.logger.Info("Successfully setup CSI driver")
	return csiDriver, nil
}

func newIdentityServer(d *s3Driver) *identityServer {
	return &identityServer{
		s3Driver: d,
	}
}

func newControllerServer(d *s3Driver) *controllerServer {
	return &controllerServer{
		s3Driver: d,
	}
}

func newNodeServer(d *s3Driver) *nodeServer {
	return &nodeServer{
		s3Driver: d,
	}
}

func (csiDriver *s3Driver) NewS3CosDriver(nodeID string, endpoint string) (*s3Driver, error) {
	s3client, err := s3client.NewS3Client()
	if err != nil {
		return nil, err
	}

	csiDriver.endpoint = endpoint
	csiDriver.s3client = s3client

	// Create GRPC servers
	csiDriver.ids = newIdentityServer(csiDriver)
	csiDriver.ns = newNodeServer(csiDriver)
	csiDriver.cs = newControllerServer(csiDriver)

	return csiDriver, nil
}

func (s3 *s3Driver) Run() {
	s3.logger.Info("-S3CSIDriver Run-")
	s3.logger.Info("Driver:", zap.Reflect("Driver Name", s3.name))
	s3.logger.Info("Version:", zap.Reflect("Driver Version", s3.vendorVersion))
	// Initialize default library driver

	s := NewNonBlockingGRPCServer(s3.logger)
	s.Start(s3.endpoint, s3.ids, s3.cs, s3.ns)
	s.Wait()
}

