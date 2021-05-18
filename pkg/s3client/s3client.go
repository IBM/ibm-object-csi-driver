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
	"fmt"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/golang/glog"
	"strings"
)

// ObjectStorageCredentials holds credentials for accessing an object storage service
type ObjectStorageCredentials struct {
	//AuthType
	AuthType string
	// AccessKey is the account identifier in AWS authentication
	AccessKey string
	// SecretKey is the "password" in AWS authentication
	SecretKey string
	// APIKey is the "password" in IBM IAM authentication
	APIKey string
	// ServiceInstanceID is the account identifier in IBM IAM authentication
	ServiceInstanceID string
	//IAMEndpoint ...
	IAMEndpoint string
}

// ObjectStorageSession is an interface of an object store session
type ObjectStorageSession interface {

	// CheckBucketAccess method check that a bucket can be accessed
	CheckBucketAccess(bucket string) error

	// CheckObjectPathExistence method checks that object-path exists inside bucket
	CheckObjectPathExistence(bucket, objectpath string) (bool, error)

	// CreateBucket methods creates a new bucket
	CreateBucket(bucket string) (string, error)

	// DeleteBucket methods deletes a bucket (with all of its objects)
	DeleteBucket(bucket string) error
}

// COSSessionFactory represents a COS (S3) session factory
type COSSessionFactory struct{}

// ObjectStorageSessionFactory is an interface of an object store session factory
type ObjectStorageSessionFactory interface {

	// NewObjectStorageBackend method creates a new object store session
	NewObjectStorageSession(endpoint, locationConstraint string, creds *ObjectStorageCredentials) ObjectStorageSession
}

// COSSession represents a COS (S3) session
type COSSession struct {
	svc s3API
}

type s3API interface {
	HeadBucket(input *s3.HeadBucketInput) (*s3.HeadBucketOutput, error)
	CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error)
	ListObjects(input *s3.ListObjectsInput) (*s3.ListObjectsOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	DeleteBucket(input *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error)
}

func (s *COSSession) CheckBucketAccess(bucket string) error {
	_, err := s.svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (s *COSSession) CheckObjectPathExistence(bucket string, objectpath string) (bool, error) {
	glog.Infof("CheckObjectPathExistence args:\n\tsbucket: <%s>\n\tobjectpath: <%s>", bucket, objectpath)
	if strings.HasPrefix(objectpath, "/") {
		objectpath = strings.TrimPrefix(objectpath, "/")
	}
	resp, err := s.svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int64(1),
		Prefix:  aws.String(objectpath),
	})
	if err != nil {
		glog.Errorf("Cannot list bucket %s", bucket)
		return false, fmt.Errorf("Cannot list bucket '%s': %v", bucket, err)
	}
	if len(resp.Contents) == 1 {
		object := *(resp.Contents[0].Key)
		if (object == objectpath) || (strings.TrimSuffix(object, "/") == objectpath) {
			return true, nil
		}
	}
	return false, nil
}

func (s *COSSession) CreateBucket(bucket string) (string, error) {
	_, err := s.svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "BucketAlreadyOwnedByYou" {
			glog.Warning(fmt.Sprintf("bucket '%s' already exists", bucket))
			return fmt.Sprintf("bucket '%s' already exists", bucket), nil
		}
		return "", err
	}

	return "", nil
}

func (s *COSSession) DeleteBucket(bucket string) error {
	resp, err := s.svc.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NoSuchBucket" {
			glog.Warning(fmt.Sprintf("bucket %s is already deleted", bucket))
			return nil
		}

		return fmt.Errorf("cannot list bucket '%s': %v", bucket, err)
	}

	for _, key := range resp.Contents {
		_, err = s.svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    key.Key,
		})

		if err != nil {
			return fmt.Errorf("cannot delete object %s/%s: %v", bucket, *key.Key, err)
		}
	}

	_, err = s.svc.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})

	return err
}

func NewS3Client() (ObjectStorageSession, error) {
	glog.Infof("NewS3Client")
	return new(COSSession), nil
}

// NewObjectStorageSession method creates a new object store session
func (s *COSSessionFactory) NewObjectStorageSession(endpoint, locationConstraint string, creds *ObjectStorageCredentials) ObjectStorageSession {
	var sdkCreds *credentials.Credentials
	if creds.AuthType == "iam" {
		sdkCreds = ibmiam.NewStaticCredentials(aws.NewConfig(), creds.APIKey, creds.ServiceInstanceID, creds.IAMEndpoint)
	} else {
		sdkCreds = credentials.NewStaticCredentials(creds.AccessKey, creds.SecretKey, "")
	}
	sess := session.Must(session.NewSession(&aws.Config{
		S3ForcePathStyle: aws.Bool(true),
		Endpoint:         aws.String(endpoint),
		Credentials:      sdkCreds,
		Region:           aws.String(locationConstraint),
	}))

	return &COSSession{
		svc: s3.New(sess),
	}
}
