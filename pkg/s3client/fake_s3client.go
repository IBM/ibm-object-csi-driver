package s3client

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

// ObjectStorageSessionFactory is a factory for mocked object storage sessions
type FakeCOSSessionFactory struct {
	FailCheckBucketAccess bool
	FailCreateBucket      bool
	FailDeleteBucket      bool
	FailBucketVersioning  bool
	FailUpdateQuotaLimit  bool
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

func (s *fakeCOSSession) CheckBucketAccess(ctx context.Context, bucket string) error {
	if s.factory.FailCheckBucketAccess {
		return errors.New("failed to check bucket access")
	}
	return nil
}

func (s *fakeCOSSession) SetBucketVersioning(ctx context.Context, bucket string, enable bool) error {
	if s.factory.FailBucketVersioning {
		return errors.New("failed to set bucket versioning")
	}
	return nil
}

func (s *fakeCOSSession) CheckObjectPathExistence(ctx context.Context, bucket, objectpath string) (bool, error) {
	return true, nil
}

func (s *fakeCOSSession) CreateBucket(ctx context.Context, bucket, kpRootKeyCrn string) (string, error) {
	if s.factory.FailCreateBucket {
		return "", errors.New("failed to create bucket")
	}
	return "", nil
}

func (s *fakeCOSSession) DeleteBucket(ctx context.Context, bucket string) error {
	if s.factory.FailDeleteBucket {
		return errors.New("failed to delete bucket")
	}
	return nil
}

func (s *fakeCOSSession) UpdateQuotaLimit(ctx context.Context, quota int64, apiKey, bucketName, cosEndpoint, iamEndpoint string) error {
	if s.factory.FailUpdateQuotaLimit {
		return errors.New("failed to update quota limit")
	}
	return nil
}
