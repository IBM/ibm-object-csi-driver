package s3client

import (
	"errors"

	"go.uber.org/zap"
)

// ObjectStorageSessionFactory is a factory for mocked object storage sessions
type FakeCOSSessionFactory struct {
	FailCheckBucketAccess bool
	FailCreateBucket      bool
	FailDeleteBucket      bool
	FailBucketVersioning  bool
}

type fakeCOSSession struct {
	factory *FakeCOSSessionFactory
}

// NewObjectStorageSession method creates a new fake object store session
func (f *FakeCOSSessionFactory) NewObjectStorageSession(endpoint, region string, creds *ObjectStorageCredentials, lgr *zap.Logger) ObjectStorageSession {
	return &fakeCOSSession{
		factory: f,
	}
}

func (s *fakeCOSSession) CheckBucketAccess(bucket string) error {
	if s.factory.FailCheckBucketAccess {
		return errors.New("failed to check bucket access")
	}
	return nil
}

func (s *fakeCOSSession) SetBucketVersioning(bucket string, enable bool) error {
	if s.factory.FailBucketVersioning {
		return errors.New("failed to set bucket versioning")
	}
	return nil
}

func (s *fakeCOSSession) CheckObjectPathExistence(bucket, objectpath string) (bool, error) {
	return true, nil
}

func (s *fakeCOSSession) CreateBucket(bucket, kpRootKeyCrn string) (string, error) {
	if s.factory.FailCreateBucket {
		return "", errors.New("failed to create bucket")
	}
	return "", nil
}

func (s *fakeCOSSession) DeleteBucket(bucket string) error {
	if s.factory.FailDeleteBucket {
		return errors.New("failed to delete bucket")
	}
	return nil
}

func (s *fakeCOSSession) UpdateQuotaLimit(quota int64, apiKey, bucketName, cosEndpoint, iamEndpoint string) error {
	return nil
}
