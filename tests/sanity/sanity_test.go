package sanity

import (
	csiDriver "github.com/IBM/satellite-object-storage-plugin/pkg/driver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	cloudProvider "github.com/IBM/ibm-csi-common/pkg/ibmcloudprovider"
	sanity "github.com/kubernetes-csi/csi-test/v4/pkg/sanity"
	"path"
	"testing"
	"fmt"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
)

func initCSIDriverForSanity(t *testing.T) *csiDriver.S3Driver {
	vendorVersion := "test-vendor-version-1.1.2"
	driver := "mydriver"

	//endpoint := "test-endpoint"
	nodeID := "test-nodeID"

	// Creating test logger
	logger, teardown := cloudProvider.GetTestLogger(t)
	defer teardown()
	// Setup the CSI driver
	icDriver, err := csiDriver.Setups3Driver("controller", driver, vendorVersion, logger)
	if err != nil {
		t.Fatalf("Failed to setup CSI Driver: %v", err)
	}
	session := NewObjectStorageSessionFactory()
	icsDriver, err := icDriver.NewS3CosDriver(nodeID, CSIEndpoint, session)
	if err != nil {
		t.Fatalf("Failed to create New COS CSI Driver: %v", err)
	}

	return icsDriver
}

var (
	// Set up variables
	TempDir = "/tmp/csi"

	// CSIEndpoint ...
	CSIEndpoint = fmt.Sprintf("unix:%s/csi.sock", TempDir)

	// TargetPath ...
	TargetPath = path.Join(TempDir, "mount")

	// StagePath ...
	StagePath = path.Join(TempDir, "stage")
)

func TestSanity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sanity testing...")
	}

	// Create a fake CSI driver
	csiSanityDriver := initCSIDriverForSanity(t)
	//  Create the temp directory for fake sanity driver
	err := os.MkdirAll(TempDir, 0755) // #nosec
	if err != nil {
		t.Fatalf("Failed to create sanity temp working dir %s: %v", TempDir, err)
	}
	defer func() {
		// Clean up tmp dir
		if err = os.RemoveAll(TempDir); err != nil {
			t.Fatalf("Failed to clean up sanity temp working dir %s: %v", TempDir, err)
		}
	}()
	fmt.Println(csiSanityDriver)
	go func() {
		csiSanityDriver.Run()
	}()

	// Run sanity test
	config := sanity.TestConfig{
		TargetPath:  TargetPath,
		StagingPath: StagePath,
		Address:     CSIEndpoint,
		DialOptions: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		SecretsFile: "../../tests/secret.yaml",
		TestVolumeParameters: map[string]string{
			"mounter": "s3fs",
			"bucket-name":  "testbucket0",
		},
	}
	sanity.Test(t, config)
	fmt.Println("here")
}



// ObjectStorageSessionFactory represents a COS (S3) session factory
type FakeObjectStorageSessionFactory struct{}

var _ s3client.ObjectStorageSessionFactory = &FakeObjectStorageSessionFactory{}

type fakeObjectStorageSession struct {
	factory *FakeObjectStorageSessionFactory
}

func NewObjectStorageSessionFactory() (*FakeObjectStorageSessionFactory) {
	return &FakeObjectStorageSessionFactory{}
}


// NewObjectStorageSession method creates a new object store session
func (f *FakeObjectStorageSessionFactory) NewObjectStorageSession(endpoint, locationConstraint string, creds *s3client.ObjectStorageCredentials) s3client.ObjectStorageSession  {
return &fakeObjectStorageSession{
		factory: f,
	}
}

func (s *fakeObjectStorageSession) CheckBucketAccess(bucket string) error {
	return nil
}

func (s *fakeObjectStorageSession) CheckObjectPathExistence(bucket, objectpath string) (bool, error) {
	return true, nil
}

func (s *fakeObjectStorageSession) CreateBucket(bucket string) (string, error) {
/*	s.factory.LastCreatedBucket = bucket
	if s.factory.FailCreateBucket {
		return "", errors.New("")
	}*/
	return "", nil
}

func (s *fakeObjectStorageSession) DeleteBucket(bucket string) error {
	return nil
}
