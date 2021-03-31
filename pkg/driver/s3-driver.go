/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

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

	csiDriver.logger.Info("Successfully setup IBM CSI driver")
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
