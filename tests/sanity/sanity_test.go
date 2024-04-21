/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package sanity ...
package sanity

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	cloudProvider "github.com/IBM/ibm-csi-common/pkg/ibmcloudprovider"
	csiDriver "github.com/IBM/ibm-object-csi-driver/pkg/driver"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/google/uuid"
	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

var (
	TempDir     = "/tmp/csi"
	CSIEndpoint = fmt.Sprintf("unix:%s/csi.sock", TempDir)
	TargetPath  = path.Join(TempDir, "mount")
	StagePath   = path.Join(TempDir, "stage")
)

func TestSanity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sanity testing...")
	}

	skipTests := strings.Join([]string{
		"CreateVolume.*should fail when requesting to create a volume with already existing name and different capacity",
		"NodeGetVolumeStats.*should fail when volume is not found",
		"NodeGetVolumeStats.*should fail when volume does not exist on the specified path", // since volume_condition is supported, so instead of err, response is sent
		"ValidateVolumeCapabilities.*should fail when the requested volume does not exist", // since volume_condition is supported, so instead of err, response is sent
	}, "|")
	err := flag.Set("ginkgo.skip", skipTests)
	if err != nil {
		t.Fatalf("Failed to set skipTests: %v, Error: %v", skipTests, err)
	}

	// Create a fake CSI driver
	csiSanityDriver := initCSIDriverForSanity(t)

	//  Create the temp directory for fake sanity driver
	err = os.MkdirAll(TempDir, 0755) // #nosec
	if err != nil {
		t.Fatalf("Failed to create sanity temp working dir %s: %v", TempDir, err)
	}
	defer func() {
		// Clean up tmp dir
		if err = os.RemoveAll(TempDir); err != nil {
			t.Fatalf("Failed to clean up sanity temp working dir %s: %v", TempDir, err)
		}
	}()

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
			"bucketName": "fakeBucketName",
			//			"mounter":    "s3fs",
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

func initCSIDriverForSanity(t *testing.T) *csiDriver.S3Driver {
	mode := "controller-node"
	driver := "fakedriver"
	vendorVersion := "fake-vendor-version-1.1.2"
	nodeID := "fakeNodeID"
	session := FakeNewObjectStorageSessionFactory()
	mountObj := FakeNewS3fsMounterFactory()
	statsUtil := &FakeNewVolumeStatsUtils{}

	// Creating test logger
	logger, teardown := cloudProvider.GetTestLogger(t)
	defer teardown()

	// Setup the CSI driver
	icDriver, err := csiDriver.Setups3Driver(mode, driver, vendorVersion, logger)
	if err != nil {
		t.Fatalf("Failed to setup CSI Driver: %v", err)
	}

	icsDriver, err := icDriver.NewS3CosDriver(nodeID, CSIEndpoint, session, mountObj, statsUtil)
	if err != nil {
		t.Fatalf("Failed to create New COS CSI Driver: %v", err)
	}

	return icsDriver
}

// Fake ObjectStorageSessionFactory
type FakeObjectStorageSessionFactory struct{}

func FakeNewObjectStorageSessionFactory() *FakeObjectStorageSessionFactory {
	return &FakeObjectStorageSessionFactory{}
}

type fakeObjectStorageSession struct {
	factory *FakeObjectStorageSessionFactory
	logger  *zap.Logger
}

func (f *FakeObjectStorageSessionFactory) NewObjectStorageSession(endpoint, locationConstraint string, creds *s3client.ObjectStorageCredentials, lgr *zap.Logger) s3client.ObjectStorageSession {
	return &fakeObjectStorageSession{
		factory: f,
		logger:  lgr,
	}
}

func (s *fakeObjectStorageSession) CheckBucketAccess(bucket string) error {
	return nil
}

func (s *fakeObjectStorageSession) CheckObjectPathExistence(bucket, objectpath string) (bool, error) {
	return true, nil
}

func (s *fakeObjectStorageSession) CreateBucket(bucket, kpRootKeyCrn string) (string, error) {
	return "", nil
}

func (s *fakeObjectStorageSession) DeleteBucket(bucket string) error {
	return nil
}

// Fake NewMounterFactory
type FakeS3fsMounterFactory struct{}

func FakeNewS3fsMounterFactory() *FakeS3fsMounterFactory {
	return &FakeS3fsMounterFactory{}
}

type Fakes3fsMounter struct {
	bucketName    string //From Secret in SC
	objPath       string //From Secret in SC
	endPoint      string //From Secret in SC
	locConstraint string //From Secret in SC
	authType      string
	accessKeys    string
	mountOptions  []string
}

func (s *FakeS3fsMounterFactory) NewMounter(attrib map[string]string, secretMap map[string]string, mountFlags []string) (mounter.Mounter, error) {
	klog.Info("-New S3FS Fake Mounter-")

	var val, accessKey, secretKey, apiKey, option string
	var check bool

	mounter := &Fakes3fsMounter{}
	options := []string{}

	if val, check = secretMap["cosEndpoint"]; check {
		mounter.endPoint = val
	}
	if val, check = secretMap["locationConstraint"]; check {
		mounter.locConstraint = val
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

	fmt.Println("$$$", mounter)

	return mounter, nil
}

func (s3fs *Fakes3fsMounter) Mount(source string, target string) error {
	klog.Info("-S3FSMounter Mount-")
	return nil
}

func (s3fs *Fakes3fsMounter) Stage(stageTarget string) error {
	klog.Info("-S3FSMounter Stage-")
	return nil
}

func (s3fs *Fakes3fsMounter) Unmount(target string) error {
	klog.Info("-S3FSMounter Unmount-")
	return nil
}

func (s3fs *Fakes3fsMounter) Unstage(stageTarget string) error {
	klog.Info("-S3FSMounter Unstage-")
	return nil
}

// For Id Generation
type providerIDGenerator struct{}

func (v providerIDGenerator) GenerateInvalidNodeID() string {
	return "invalid-Node-ID"
}

func (v providerIDGenerator) GenerateInvalidVolumeID() string {
	return "invalid-vol-ID"
}

func (v providerIDGenerator) GenerateUniqueValidNodeID() string {
	return fmt.Sprintf("fake-node-ID-%s", uuid.New().String()[:10])
}

func (v providerIDGenerator) GenerateUniqueValidVolumeID() string {
	return fmt.Sprintf("fake-vol-ID-%s", uuid.New().String()[:10])
}

// Fake VolumeStatsUtils
type FakeNewVolumeStatsUtils struct {
}

func (su *FakeNewVolumeStatsUtils) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	if path == "some/path" {
		return 0, 0, 0, 0, 0, 0, status.Error(codes.NotFound, "volume not found on some/path")
	}
	return 1, 1, 1, 1, 1, 1, nil
}

func (su *FakeNewVolumeStatsUtils) CheckMount(targetPath string) (bool, error) {
	return true, nil
}

func (su *FakeNewVolumeStatsUtils) FuseUnmount(path string) error {
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
