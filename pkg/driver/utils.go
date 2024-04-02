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
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/kubernetes/pkg/volume/util/fs"
)

func ReplaceAndReturnCopy(req interface{}, newAccessKey, newSecretKey string) (interface{}, error) {
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
			if k == "accessKey" || k == "secretKey" || k == "apiKey" {
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
			if k == "accessKey" || k == "secretKey" || k == "apiKey" {
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
			if k == "accessKey" || k == "secretKey" || k == "apiKey" {
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

type statsUtils interface {
	FSInfo(path string) (int64, int64, int64, int64, int64, int64, error)
	// IsBlockDevice(devicePath string) (bool, error)
	// DeviceInfo(devicePath string) (int64, error)
	// IsDevicePathNotExist(devicePath string) bool
}

type VolumeStatsUtils struct {
}

func (su *VolumeStatsUtils) FSInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	return fs.Info(path)
}
