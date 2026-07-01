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
	"context"
	"fmt"
	"strings"

	"github.com/IBM/go-sdk-core/v5/core"
	rc "github.com/IBM/ibm-cos-sdk-go-config/v2/resourceconfigurationv1"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
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
	CheckBucketAccess(ctx context.Context, bucket string) error

	// CheckObjectPathExistence method checks that object-path exists inside bucket
	CheckObjectPathExistence(ctx context.Context, bucket, objectpath string) (bool, error)

	// CreateBucket methods creates a new bucket
	CreateBucket(ctx context.Context, bucket, kpRootKeyCrn string) (string, error)

	// DeleteBucket methods deletes a bucket (with all of its objects)
	DeleteBucket(ctx context.Context, bucket string) error

	SetBucketVersioning(ctx context.Context, bucket string, enable bool) error

	UpdateQuotaLimit(ctx context.Context, quota int64, apiKey, bucketName, cosEndpoint, iamEndpoint string) error
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
	logger          *zap.Logger
	svc             s3API
	rcClientFactory rcClientFactory
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

type rcAPI interface {
	UpdateBucketConfig(options *rc.UpdateBucketConfigOptions) (*core.DetailedResponse, error)
}

type rcClientFactory interface {
	NewResourceConfigurationV1(options *rc.ResourceConfigurationV1Options) (rcAPI, error)
}

type defaultRCClientFactory struct{}

func (f *defaultRCClientFactory) NewResourceConfigurationV1(options *rc.ResourceConfigurationV1Options) (rcAPI, error) {
	return rc.NewResourceConfigurationV1(options)
}

func (s *COSSession) CheckBucketAccess(ctx context.Context, bucket string) error {
	reqID := requestid.FromContext(ctx)
	log := s.logger.With(zap.String("request_id", reqID))

	log.Info("CheckBucketAccess started", zap.String("bucket", bucket))
	_, err := s.svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		log.Error("CheckBucketAccess failed", zap.String("bucket", bucket), zap.Error(err))
	} else {
		log.Info("CheckBucketAccess completed", zap.String("bucket", bucket))
	}
	return err
}

func (s *COSSession) CheckObjectPathExistence(ctx context.Context, bucket string, objectpath string) (bool, error) {
	reqID := requestid.FromContext(ctx)
	log := s.logger.With(zap.String("request_id", reqID))

	log.Info("CheckObjectPathExistence started",
		zap.String("bucket", bucket), zap.String("objectpath", objectpath))

	objectpath = strings.TrimPrefix(objectpath, "/")
	if !strings.HasSuffix(objectpath, "/") {
		objectpath = objectpath + "/"
	}

	resp, err := s.svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int64(1),
		Prefix:  aws.String(objectpath),
	})
	if err != nil {
		log.Error("Cannot list bucket", zap.String("bucket", bucket), zap.Error(err))
		return false, fmt.Errorf("cannot list bucket '%s': %v", bucket, err)
	}

	exists := false
	if len(resp.Contents) == 1 {
		object := *(resp.Contents[0].Key)
		if (object == objectpath) || (strings.TrimSuffix(object, "/") == objectpath) {
			exists = true
		}
	}

	log.Info("CheckObjectPathExistence completed",
		zap.String("bucket", bucket), zap.String("objectpath", objectpath), zap.Bool("exists", exists))
	return exists, nil
}

func (s *COSSession) CreateBucket(ctx context.Context, bucket, kpRootKeyCrn string) (res string, err error) {
	reqID := requestid.FromContext(ctx)
	log := s.logger.With(zap.String("request_id", reqID))

	log.Info("CreateBucket started",
		zap.String("bucket", bucket),
		zap.Bool("encryption_enabled", kpRootKeyCrn != ""))

	if kpRootKeyCrn != "" {
		log.Debug("Creating bucket with KP encryption", zap.String("bucket", bucket))
		_, err = s.svc.CreateBucket(&s3.CreateBucketInput{
			Bucket:                      aws.String(bucket),
			IBMSSEKPCustomerRootKeyCrn:  aws.String(kpRootKeyCrn),
			IBMSSEKPEncryptionAlgorithm: aws.String(constants.KPEncryptionAlgorithm),
		})
	} else {
		log.Debug("Creating bucket without encryption", zap.String("bucket", bucket))
		_, err = s.svc.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
	}

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "BucketAlreadyOwnedByYou" {
			log.Warn("Bucket already exists", zap.String("bucket", bucket))
			return fmt.Sprintf("bucket '%s' already exists", bucket), nil
		}
		log.Error("CreateBucket failed", zap.String("bucket", bucket), zap.Error(err))
		return "", err
	}

	log.Info("CreateBucket completed successfully", zap.String("bucket", bucket))
	return "", nil
}

func (s *COSSession) DeleteBucket(ctx context.Context, bucket string) error {
	reqID := requestid.FromContext(ctx)
	log := s.logger.With(zap.String("request_id", reqID))

	log.Info("DeleteBucket started", zap.String("bucket", bucket))

	resp, err := s.svc.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NoSuchBucket" {
			log.Warn("Bucket already deleted", zap.String("bucket", bucket))
			return nil
		}
		log.Error("Cannot list bucket", zap.String("bucket", bucket), zap.Error(err))
		return fmt.Errorf("cannot list bucket '%s': %v", bucket, err)
	}

	objectCount := len(resp.Contents)
	log.Info("Deleting objects from bucket",
		zap.String("bucket", bucket), zap.Int("object_count", objectCount))

	for _, key := range resp.Contents {
		_, err = s.svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    key.Key,
		})

		if err != nil {
			log.Error("Cannot delete object",
				zap.String("bucket", bucket), zap.String("key", *key.Key), zap.Error(err))
			return fmt.Errorf("cannot delete object %s/%s: %v", bucket, *key.Key, err)
		}
	}

	log.Info("Deleting bucket", zap.String("bucket", bucket))
	_, err = s.svc.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})

	if err != nil {
		log.Error("DeleteBucket failed", zap.String("bucket", bucket), zap.Error(err))
	} else {
		log.Info("DeleteBucket completed successfully", zap.String("bucket", bucket))
	}
	return err
}

func (s *COSSession) SetBucketVersioning(ctx context.Context, bucket string, enable bool) error {
	reqID := requestid.FromContext(ctx)
	log := s.logger.With(zap.String("request_id", reqID))

	status := s3.BucketVersioningStatusSuspended
	if enable {
		status = s3.BucketVersioningStatusEnabled
	}

	log.Info("SetBucketVersioning started",
		zap.String("bucket", bucket), zap.Bool("enable", enable))

	_, err := s.svc.PutBucketVersioning(&s3.PutBucketVersioningInput{
		Bucket: aws.String(bucket),
		VersioningConfiguration: &s3.VersioningConfiguration{
			Status: aws.String(status),
		},
	})
	if err != nil {
		log.Error("SetBucketVersioning failed",
			zap.String("bucket", bucket), zap.Bool("enable", enable), zap.Error(err))
		return fmt.Errorf("failed to set versioning to %v for bucket '%s': %v", enable, bucket, err)
	}

	log.Info("SetBucketVersioning completed successfully",
		zap.String("bucket", bucket), zap.Bool("enable", enable))
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
		svc:             s3.New(sess),
		logger:          lgr,
		rcClientFactory: &defaultRCClientFactory{},
	}
}

func (s *COSSession) UpdateQuotaLimit(ctx context.Context, quota int64, apiKey, bucketName, cosEndpoint, iamEndpoint string) error {
	reqID := requestid.FromContext(ctx)
	log := s.logger.With(zap.String("request_id", reqID))

	log.Info("UpdateQuotaLimit started",
		zap.String("bucket", bucketName), zap.Int64("quota", quota))

	var configEndpoint string
	if strings.Contains(strings.ToLower(cosEndpoint), "private") {
		configEndpoint = constants.ResourceConfigEPPrivate
		log.Debug("Using private resource config endpoint")
	} else {
		configEndpoint = constants.ResourceConfigEPDirect
		log.Debug("Using direct resource config endpoint")
	}

	iamTokenURL := iamEndpoint + "/identity/token"

	authenticator := &core.IamAuthenticator{
		ApiKey: apiKey,
		URL:    iamTokenURL,
	}

	log.Debug("Creating resource configuration service")
	service, err := s.rcClientFactory.NewResourceConfigurationV1(&rc.ResourceConfigurationV1Options{
		Authenticator: authenticator,
		URL:           configEndpoint,
	})
	if err != nil {
		log.Error("Failed to create resource configuration service", zap.Error(err))
		return fmt.Errorf("failed to create resource configuration service: %w", err)
	}

	bucketPatch := make(map[string]interface{})
	bucketPatch["hard_quota"] = core.Int64Ptr(quota)

	options := &rc.UpdateBucketConfigOptions{
		Bucket:      core.StringPtr(bucketName),
		BucketPatch: bucketPatch,
	}

	log.Info("Updating bucket quota",
		zap.String("bucket", bucketName), zap.Int64("quota", quota))
	_, err = service.UpdateBucketConfig(options)
	if err != nil {
		log.Error("Failed to update quota",
			zap.String("bucket", bucketName), zap.Int64("quota", quota), zap.Error(err))
		return fmt.Errorf("failed to update quota for bucket %s to %d bytes: %w", bucketName, quota, err)
	}

	log.Info("UpdateQuotaLimit completed successfully",
		zap.String("bucket", bucketName), zap.Int64("quota", quota))
	return nil
}
