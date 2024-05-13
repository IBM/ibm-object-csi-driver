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
package fake

import (
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type fakercloneMounter struct {
	bucketName    string //From Secret in SC
	objPath       string //From Secret in SC
	endPoint      string //From Secret in SC
	locConstraint string //From Secret in SC
	authType      string
	accessKeys    string
	kpRootKeyCrn  string
	uid           string
	gid           string
}

func fakenewRcloneMounter(bucket string, objpath string, endpoint string, region string, keys string, authType, kpCrn, uid, gid string) (mounter.Mounter, error) {
	klog.Infof("-newS3fsMounter-")
	klog.Infof("newS3fsMounter args:\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>\n\tkeys: <%s>", bucket, objpath, endpoint, region, keys)
	return &fakercloneMounter{
		bucketName:    bucket,
		objPath:       objpath,
		endPoint:      endpoint,
		locConstraint: region,
		accessKeys:    keys,
		authType:      authType,
		kpRootKeyCrn:  kpCrn,
		uid:           uid,
		gid:           gid,
	}, nil
}

func (s3fs *fakercloneMounter) Mount(source string, target string) error {
	return nil
}

func (s3fs *fakercloneMounter) Unmount(target string) error {
	return nil
}
