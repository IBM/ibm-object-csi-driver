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
	"strings"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"go.uber.org/zap"
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
	// KpRootKeyCrn
	KpRootKeyCRN string
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
	CreateBucket(bucket, kpRootKeyCrn string) (string, error)

	// DeleteBucket methods deletes a bucket (with all of its objects)
	DeleteBucket(bucket string) error

	EnableBucketVersioning(bucket string) error
}

// COSSessionFactory represents a COS (S3) session factory
type COSSessionFactory struct{}

// ObjectStorageSessionFactory is an interface of an object store session factory
type ObjectStorageSessionFactory interface {
	// NewObjectStorageBackend method creates a new object store session
	NewObjectStorageSession(endpoint, locationConstraint string, creds *ObjectStorageCredentials, lgr *zap.Logger) ObjectStorageSession
}

var _ ObjectStorageSessionFactory = &COSSessionFactory{}

// COSSession represents a COS (S3) session
type COSSession struct {
	logger *zap.Logger
	svc    s3API
}

func NewObjectStorageSessionFactory() *COSSessionFactory {
	return &COSSessionFactory{}
}

type s3API interface {
	HeadBucket(input *s3.HeadBucketInput) (*s3.HeadBucketOutput, error)
	CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error)
	ListObjects(input *s3.ListObjectsInput) (*s3.ListObjectsOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	DeleteBucket(input *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error)
	PutBucketVersioning(input *s3.PutBucketVersioningInput) (*s3.PutBucketVersioningOutput, error)
}

func (s *COSSession) CheckBucketAccess(bucket string) error {
	_, err := s.svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (s *COSSession) CheckObjectPathExistence(bucket string, objectpath string) (bool, error) {
	s.logger.Info("CheckObjectPathExistence args", zap.String("bucket", bucket), zap.String("objectpath", objectpath))
	objectpath = strings.TrimPrefix(objectpath, "/")
	resp, err := s.svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int64(1),
		Prefix:  aws.String(objectpath),
	})
	if err != nil {
		s.logger.Error("cannot list bucket", zap.String("bucket", bucket))
		return false, fmt.Errorf("cannot list bucket '%s': %v", bucket, err)
	}
	if len(resp.Contents) == 1 {
		object := *(resp.Contents[0].Key)
		if (object == objectpath) || (strings.TrimSuffix(object, "/") == objectpath) {
			return true, nil
		}
	}
	return false, nil
}

func (s *COSSession) CreateBucket(bucket, kpRootKeyCrn string) (res string, err error) {
	if kpRootKeyCrn != "" {
		_, err = s.svc.CreateBucket(&s3.CreateBucketInput{
			Bucket:                      aws.String(bucket),
			IBMSSEKPCustomerRootKeyCrn:  aws.String(kpRootKeyCrn),
			IBMSSEKPEncryptionAlgorithm: aws.String(constants.KPEncryptionAlgorithm),
		})
	} else {
		_, err = s.svc.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
	}

	if err != nil {
		// TODO
		// CreateVolume: Unable to create the bucket: %BucketAlreadyExists:
		// The requested bucket name is not available. The bucket namespace is shared by all users of the system.
		// Please select a different name and try again.

		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "BucketAlreadyOwnedByYou" {
			s.logger.Warn("bucket already exists", zap.String("bucket", bucket))
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
			s.logger.Warn("bucket already deleted", zap.String("bucket", bucket))
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

func (s *COSSession) EnableBucketVersioning(bucket string) error {
	s.logger.Info("Enabling versioning for bucket", zap.String("bucket", bucket))
	_, err := s.svc.PutBucketVersioning(&s3.PutBucketVersioningInput{
		Bucket: aws.String(bucket),
		VersioningConfiguration: &s3.VersioningConfiguration{
			Status: aws.String("Enabled"),
		},
	})
	if err != nil {
		s.logger.Error("Failed to enable versioning", zap.String("bucket", bucket), zap.Error(err))
		return fmt.Errorf("failed to enable versioning for bucket '%s': %v", bucket, err)
	}
	s.logger.Info("Versioning enabled successfully for bucket", zap.String("bucket", bucket))
	return nil
}

func NewS3Client(lgr *zap.Logger) (ObjectStorageSession, error) {
	cosSession := new(COSSession)
	cosSession.logger = lgr
	lgr.Info("--NewS3Client--")
	return cosSession, nil
}

// NewObjectStorageSession method creates a new object store session
func (s *COSSessionFactory) NewObjectStorageSession(endpoint, locationConstraint string, creds *ObjectStorageCredentials, lgr *zap.Logger) ObjectStorageSession {
	var sdkCreds *credentials.Credentials
	if creds.AuthType == "iam" {
		sdkCreds = ibmiam.NewStaticCredentials(aws.NewConfig(), creds.IAMEndpoint+"/identity/token", creds.APIKey, creds.ServiceInstanceID)
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
		svc:    s3.New(sess),
		logger: lgr,
	}
}
