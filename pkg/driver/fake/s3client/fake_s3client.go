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

package s3client

import (
	"errors"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
)

//ObjectStorageSessionFactory is a factory for mocked object storage sessions
type ObjectStorageSessionFactory struct {
	//FailCheckBucketAccess ...
	FailCheckBucketAccess bool
	//FailCreateBucket ...
	FailCreateBucket bool
	//FailDeleteBucket ...
	FailDeleteBucket bool
	//CheckObjectPathExistenceError ...
	CheckObjectPathExistenceError bool
	//CheckObjectPathExistencePathNotFound ...
	CheckObjectPathExistencePathNotFound bool

	// LastEndpoint holds the endpoint of the last created session
	LastEndpoint string
	// LastRegion holds the region of the last created session
	LastRegion string
	// LastCredentials holds the credentials of the last created session
	LastCredentials *s3client.ObjectStorageCredentials
	// LastCheckedBucket stores the name of the last bucket that was checked
	LastCheckedBucket string
	// LastCreatedBucket stores the name of the last bucket that was created
	LastCreatedBucket string
	// LastDeletedBucket stores the name of the last bucket that was deleted
	LastDeletedBucket string
}

type fakeObjectStorageSession struct {
	factory *ObjectStorageSessionFactory
}

// NewObjectStorageSession method creates a new fake object store session
func (f *ObjectStorageSessionFactory) NewObjectStorageSession(endpoint, region string, creds *s3client.ObjectStorageCredentials) s3client.ObjectStorageSession {
	f.LastEndpoint = endpoint
	f.LastRegion = region
	f.LastCredentials = creds
	return &fakeObjectStorageSession{
		factory: f,
	}
}

// ResetStats clears the details about previous sessions
func (f *ObjectStorageSessionFactory) ResetStats() {
	f.LastEndpoint = ""
	f.LastRegion = ""
	f.LastCredentials = &s3client.ObjectStorageCredentials{}
	f.LastCheckedBucket = ""
	f.LastCreatedBucket = ""
	f.LastDeletedBucket = ""
}

func (s *fakeObjectStorageSession) CheckBucketAccess(bucket string) error {
	s.factory.LastCheckedBucket = bucket
	if s.factory.FailCheckBucketAccess {
		return errors.New("")
	}
	return nil
}

func (s *fakeObjectStorageSession) CheckObjectPathExistence(bucket, objectpath string) (bool, error) {
	if s.factory.CheckObjectPathExistenceError {
		return false, errors.New("")
	} else if s.factory.CheckObjectPathExistencePathNotFound {
		return false, nil
	}
	return true, nil
}

func (s *fakeObjectStorageSession) CreateBucket(bucket string) (string, error) {
	s.factory.LastCreatedBucket = bucket
	if s.factory.FailCreateBucket {
		return "", errors.New("")
	}
	return "", nil
}

func (s *fakeObjectStorageSession) DeleteBucket(bucket string) error {
	s.factory.LastDeletedBucket = bucket
	if s.factory.FailDeleteBucket {
		return errors.New("")
	}
	return nil
}
