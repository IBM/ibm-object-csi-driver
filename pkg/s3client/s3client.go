/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package s3client

import (
	"fmt"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/golang/glog"
	"strings"
)

type S3Credentials struct {
	authType    string
	accessKey   string
	secretKey   string
	apiKey      string
	svcInstID   string
	iamEndpoint string
}

//S3 Client Interface
type S3Client interface {
	InitSession(endpoint string, class string, creds *S3Credentials) error
	CheckBucketAccess(bucket string) error
	CheckObjectPathExistence(bucket, objectpath string) (bool, error)
	CreateBucket(bucket string) (string, error)
	DeleteBucket(bucket string) error
}

type s3IbmApi interface {
	HeadBucket(input *s3.HeadBucketInput) (*s3.HeadBucketOutput, error)
	CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error)
	ListObjects(input *s3.ListObjectsInput) (*s3.ListObjectsOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	DeleteBucket(input *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error)
}

func (cred *S3Credentials) SetCreds(authtype string, accesskey string, secretkey string, apikey string, svcid string, iamep string) {
	cred.authType = authtype
	cred.accessKey = accesskey
	cred.secretKey = secretkey
	cred.apiKey = apikey
	cred.svcInstID = svcid
	cred.iamEndpoint = iamep
}

const (
	awsS3Type = "awss3"
)

//IBMS3Client Implements s3Client
type IbmS3Client struct {
	s3ApiCred *credentials.Credentials
	s3API     s3IbmApi
}

func (client *IbmS3Client) CheckBucketAccess(bucket string) error {
	_, err := client.s3API.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (client *IbmS3Client) CheckObjectPathExistence(bucket string, objectpath string) (bool, error) {
	glog.Infof("CheckObjectPathExistence args:\n\tsbucket: <%s>\n\tobjectpath: <%s>", bucket, objectpath)
	if strings.HasPrefix(objectpath, "/") {
		objectpath = strings.TrimPrefix(objectpath, "/")
	}
	resp, err := client.s3API.ListObjectsV2(&s3.ListObjectsV2Input{
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

func (client *IbmS3Client) CreateBucket(bucket string) (string, error) {
	return "", nil
}

func (client *IbmS3Client) DeleteBucket(bucket string) error {
	return nil
}

func NewS3Client(client string) (S3Client, error) {
	glog.Infof("NewS3Client args: :\n\tclient: <%s>", client)
	switch client {
	case awsS3Type:
		return new(IbmS3Client), nil
	default:
		return new(IbmS3Client), nil
	}
}

func (client *IbmS3Client) InitSession(endpoint string, class string, creds *S3Credentials) error {
	glog.Infof("AWSS3Client args:\n\tendpoint: <%s>\n\tclass: <%s>", endpoint, class)
	if creds.authType == "hmac" {
		client.s3ApiCred = credentials.NewStaticCredentials(creds.accessKey, creds.secretKey, "")
	}
	if creds.authType == "iam" {
		client.s3ApiCred = ibmiam.NewStaticCredentials(aws.NewConfig(), creds.iamEndpoint, creds.apiKey, creds.svcInstID)
	}

	//session.New Deprecated use session.NewSession
	//https://docs.aws.amazon.com/sdk-for-go/api/aws/session/#NewSession
	//https://docs.aws.amazon.com/sdk-for-go/api/aws/session/#Must
	sessn := session.Must(session.NewSession(&aws.Config{
		S3ForcePathStyle: aws.Bool(true),
		Endpoint:         aws.String(endpoint),
		Credentials:      client.s3ApiCred,
		Region:           aws.String(class),
	}))

	//https://docs.aws.amazon.com/sdk-for-go/api/service/s3/#New
	client.s3API = s3.New(sessn)

	return nil
}
