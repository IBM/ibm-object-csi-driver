/**
 * Copyright 2023 IBM Corp.
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

package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

func getCredentials(secretMap map[string]string) (*s3client.ObjectStorageCredentials, error) {
	var (
		accessKey         string
		secretKey         string
		apiKey            string
		serviceInstanceID string
		authType          string
		iamEndpoint       string
	)

	if val, check := secretMap["iamEndpoint"]; check {
		iamEndpoint = val
	}
	if iamEndpoint == "" {
		iamEndpoint = constants.DefaultIAMEndPoint
	}

	if val, check := secretMap["apiKey"]; check {
		apiKey = val // pragma: allowlist secret
	}

	if apiKey == "" {
		authType = "hmac"
		accessKey = secretMap["accessKey"]
		if accessKey == "" {
			return nil, status.Error(codes.Unauthenticated, "Valid access credentials are not provided in the secret| accessKey unknown")
		}

		secretKey = secretMap["secretKey"]
		if secretKey == "" {
			return nil, status.Error(codes.Unauthenticated, "Valid access credentials are not provided in the secret| secretKey unknown")
		}
	} else {
		authType = "iam"
		serviceInstanceID = secretMap["serviceId"]
		if serviceInstanceID == "" {
			return nil, status.Error(codes.Unauthenticated, "Valid access credentials are not provided in the secret| serviceId  unknown")
		}
	}

	return &s3client.ObjectStorageCredentials{
		AuthType:          authType,
		AccessKey:         accessKey,
		SecretKey:         secretKey,
		APIKey:            apiKey, // pragma: allowlist secret
		IAMEndpoint:       iamEndpoint,
		ServiceInstanceID: serviceInstanceID,
	}, nil
}

func getTempBucketName(mounterType, volumeID string) string {
	currentTime := time.Now()
	timestamp := currentTime.Format("20060102150405")

	name := fmt.Sprintf("%s%s-%s", mounterType, timestamp, volumeID)
	return name
}

func createBucket(sess s3client.ObjectStorageSession, bucketName, kpRootKeyCrn string) error {
	msg, err := sess.CreateBucket(bucketName, kpRootKeyCrn)
	if msg != "" {
		klog.Infof("Info:Create Volume module with user provided Bucket name: %v", msg)
	}
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "BucketAlreadyExists" {
			klog.Warning(fmt.Sprintf("bucket '%s' already exists", bucketName))
		} else {
			klog.Errorf("CreateVolume: Unable to create the bucket: %v", err)
			return errors.New("unable to create the bucket")
		}
	}
	if err := sess.CheckBucketAccess(bucketName); err != nil {
		klog.Errorf("CreateVolume: Unable to access the bucket: %v", err)
		return errors.New("unable to access the bucket")
	}
	return nil
}

func sanitizeVolumeID(volumeID string) (string, error) {
	var err error
	volumeID = strings.ToLower(volumeID)
	if len(volumeID) > 63 {
		h := sha256.New()
		_, err = io.WriteString(h, volumeID) //nolint
		volumeID = hex.EncodeToString(h.Sum(nil))
	}
	return volumeID, err
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range volumeCapabilities {
			volumeCap := csi.VolumeCapability_AccessMode{
				Mode: c,
			}
			if volumeCap.GetMode() == cap.AccessMode.GetMode() {
				return true
			}
		}
		return false
	}

	foundAll := true
	for _, c := range volCaps {
		if !hasSupport(c) {
			foundAll = false
		}
	}
	return foundAll
}

func ReplaceAndReturnCopy(req interface{}) (interface{}, error) {
	switch r := req.(type) {
	case *csi.CreateVolumeRequest:
		// Create a new CreateVolumeRequest and copy the original values
		var inReq *csi.CreateVolumeRequest

		newReq := &csi.CreateVolumeRequest{}
		*newReq = *r

		inReq = req.(*csi.CreateVolumeRequest)

		// Modify the Secrets map in the new request
		newReq.Secrets = make(map[string]string)
		secretMap := inReq.GetSecrets()

		for k, v := range secretMap {
			if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
				newReq.Secrets[k] = "xxxxxxx"
				continue
			}
			newReq.Secrets[k] = v
		}
		return newReq, nil
	case *csi.DeleteVolumeRequest:
		// Create a new DeleteVolumeRequest and copy the original values
		var inReq *csi.DeleteVolumeRequest

		newReq := &csi.DeleteVolumeRequest{}
		*newReq = *r

		inReq = req.(*csi.DeleteVolumeRequest)

		// Modify the Secrets map in the new request
		newReq.Secrets = make(map[string]string)
		secretMap := inReq.GetSecrets()

		for k, v := range secretMap {
			if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
				newReq.Secrets[k] = "xxxxxxx"
				continue
			}
			newReq.Secrets[k] = v
		}

		return newReq, nil
	case *csi.NodePublishVolumeRequest:
		// Create a new NodePublishVolumeRequest and copy the original values
		var inReq *csi.NodePublishVolumeRequest

		newReq := &csi.NodePublishVolumeRequest{}
		*newReq = *r

		inReq = req.(*csi.NodePublishVolumeRequest)

		// Modify the Secrets map in the new request
		newReq.Secrets = make(map[string]string)
		secretMap := inReq.GetSecrets()

		for k, v := range secretMap {
			if k == "accessKey" || k == "secretKey" || k == "apiKey" || k == "kpRootKeyCRN" {
				newReq.Secrets[k] = "xxxxxxx"
				continue
			}
			newReq.Secrets[k] = v
		}

		return newReq, nil

	default:
		return req, fmt.Errorf("unsupported request type")
	}
}
