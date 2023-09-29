package sanity

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	cloudProvider "github.com/IBM/ibm-csi-common/pkg/ibmcloudprovider"
	csiDriver "github.com/IBM/satellite-object-storage-plugin/pkg/driver"
	"github.com/IBM/satellite-object-storage-plugin/pkg/mounter"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
	"github.com/google/uuid"
	sanity "github.com/kubernetes-csi/csi-test/v4/pkg/sanity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
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
	icDriver, err := csiDriver.Setups3Driver("controller-node", driver, vendorVersion, logger)
	if err != nil {
		t.Fatalf("Failed to setup CSI Driver: %v", err)
	}
	session := NewObjectStorageSessionFactory()
	icsDriver, err := icDriver.NewS3CosDriver(nodeID, CSIEndpoint, session, FakeNewS3fsMounterFactory())
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

const (

	// FakeNodeID
	FakeNodeID = "fake-node-id"
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
			"mounter":    "s3fs",
			"bucketName": "testbucket0",
		},
		CreateTargetDir: func(targetPath string) (string, error) {
			return targetPath, createTargetDir(targetPath)
		},
		CreateStagingDir: func(stagePath string) (string, error) {
			return stagePath, createTargetDir(stagePath)
		},
		IDGen: &providerIDGenerator{},
	}
	sanity.Test(t, config)
}

// ObjectStorageSessionFactory represents a COS (S3) session factory
type FakeObjectStorageSessionFactory struct{}

var _ s3client.ObjectStorageSessionFactory = &FakeObjectStorageSessionFactory{}

type fakeObjectStorageSession struct {
	factory *FakeObjectStorageSessionFactory
}

func NewObjectStorageSessionFactory() *FakeObjectStorageSessionFactory {
	return &FakeObjectStorageSessionFactory{}
}

// NewObjectStorageSession method creates a new object store session
func (f *FakeObjectStorageSessionFactory) NewObjectStorageSession(endpoint, locationConstraint string, creds *s3client.ObjectStorageCredentials) s3client.ObjectStorageSession {
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

func createTargetDir(targetPath string) error {
	fileInfo, err := os.Stat(targetPath)
	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(targetPath, 0755)
	} else if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("target location %s is not a directory", targetPath)
	}

	return nil
}

type FakeS3fsMounterFactory struct{}

type Fakes3fsMounter struct {
	bucketName   string //From Secret in SC
	objPath      string //From Secret in SC
	endPoint     string //From Secret in SC
	regnClass    string //From Secret in SC
	authType     string
	accessKeys   string
	mountOptions []string
}

func (s *FakeS3fsMounterFactory) NewMounter(secretMap map[string]string, mountFlags []string) (mounter.Mounter, error) {
	klog.Info("-New S3FS Fake Mounter-")

	var (
		val       string
		check     bool
		accessKey string
		secretKey string
		apiKey    string
		mounter   *Fakes3fsMounter
		options   []string
	)

	mounter = &Fakes3fsMounter{}
	options = []string{}

	if val, check = secretMap["cosEndpoint"]; check {
		mounter.bucketName = val
	}
	if val, check = secretMap["regionClass"]; check {
		mounter.regnClass = val
	}
	if val, check = secretMap["bucketName"]; check {
		mounter.bucketName = val
	}
	if val, check = secretMap["objPath"]; check {
		mounter.objPath = val
	}
	if val, check = secretMap["accessKey"]; check {
		accessKey = val
	}
	if val, check = secretMap["secretKey"]; check {
		secretKey = val
	}
	if val, check = secretMap["apiKey"]; check {
		apiKey = val
	}
	if apiKey != "" {
		mounter.accessKeys = fmt.Sprintf(":%s", apiKey)
		mounter.authType = "iam"
	} else {
		mounter.accessKeys = fmt.Sprintf("%s:%s", accessKey, secretKey)
		mounter.authType = "hmac"
	}

	var option string
	for _, val = range mountFlags {
		option = val
		isKeyValuePair := true
		keys := strings.Split(val, "=")
		if len(keys) == 2 {
			if keys[0] == "cache" {
				isKeyValuePair = false
				option = keys[1]
			}
			if newVal, check := secretMap[keys[0]]; check {
				if isKeyValuePair {
					option = fmt.Sprintf("%s=%s", keys[0], newVal)
				} else {
					option = newVal
				}
			}
		}
		options = append(options, option)
		klog.Infof("NewMounter mountOption: [%s]", option)
	}
	if val, check = secretMap["tmpdir"]; check {
		option = fmt.Sprintf("tmpdir=%s", val)
		options = append(options, option)
	}
	if val, check = secretMap["use_cache"]; check {
		option = fmt.Sprintf("use_cache=%s", val)
		options = append(options, option)
	}
	mounter.mountOptions = options

	return mounter, nil
}

func (s3fs *Fakes3fsMounter) Stage(stageTarget string) error {
	klog.Info("-S3FSMounter Stage-")
	return nil
}

func (s3fs *Fakes3fsMounter) Unstage(stageTarget string) error {
	klog.Info("-S3FSMounter Unstage-")
	return nil
}

func (s3fs *Fakes3fsMounter) Unmount(target string) error {
	klog.Info("-S3FSMounter Unmount-")
	return nil
}

func (s3fs *Fakes3fsMounter) Mount(source string, target string) error {
	klog.Info("-S3FSMounter Mount-")
	return nil
}

func FakeNewS3fsMounterFactory() *FakeS3fsMounterFactory {
	return &FakeS3fsMounterFactory{}
}

// For Id Generation

var _ sanity.IDGenerator = &providerIDGenerator{}

type providerIDGenerator struct {
}

func (v providerIDGenerator) GenerateUniqueValidVolumeID() string {
	return fmt.Sprintf("vol-uuid-test-vol-%s", uuid.New().String()[:10])
}

func (v providerIDGenerator) GenerateInvalidVolumeID() string {
	return "invalid-vol-id"
}

func (v providerIDGenerator) GenerateUniqueValidNodeID() string {
	return fmt.Sprintf("%s-%s", FakeNodeID, uuid.New().String()[:10])
}

func (v providerIDGenerator) GenerateInvalidNodeID() string {
	return "invalid-Node-ID"
}
