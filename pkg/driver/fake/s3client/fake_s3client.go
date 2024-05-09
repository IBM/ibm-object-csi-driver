/**
 * Copyright 2024 IBM Corp.
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

package s3client

import (
	"errors"

	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"go.uber.org/zap"
)

// ObjectStorageSessionFactory is a factory for mocked object storage sessions
type FakeCOSSessionFactory struct {
	FailCheckBucketAccess bool
	FailCreateBucket      bool
	FailDeleteBucket      bool
}

type fakeCOSSession struct {
	factory *FakeCOSSessionFactory
	logger  *zap.Logger
}

// NewObjectStorageSession method creates a new fake object store session
func (f *FakeCOSSessionFactory) NewObjectStorageSession(endpoint, region string, creds *s3client.ObjectStorageCredentials, lgr *zap.Logger) s3client.ObjectStorageSession {
	return &fakeCOSSession{
		factory: f,
		logger:  lgr,
	}
}

func (s *fakeCOSSession) CheckBucketAccess(bucket string) error {
	if s.factory.FailCheckBucketAccess {
		return errors.New("failed to check bucket access")
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
