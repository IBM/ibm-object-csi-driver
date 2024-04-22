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
package fake

import (
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	"k8s.io/klog/v2"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type fakes3fsMounter struct {
	bucketName string //From Secret in SC
	objPath    string //From Secret in SC
	endPoint   string //From Secret in SC
	regnClass  string //From Secret in SC
	accessKeys string
	authType   string
}

func fakenewS3fsMounter(bucket string, objpath string, endpoint string, region string, keys string, authType string) (mounter.Mounter, error) {
	klog.Infof("-newS3fsMounter-")
	klog.Infof("newS3fsMounter args:\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>\n\tkeys: <%s>", bucket, objpath, endpoint, region, keys)
	return &fakes3fsMounter{
		bucketName: bucket,
		objPath:    objpath,
		endPoint:   endpoint,
		regnClass:  region,
		accessKeys: keys,
		authType:   authType,
	}, nil
}

func (s3fs *fakes3fsMounter) Stage(stageTarget string) error {
	klog.Infof("-S3FSMounter Stage-")
	return nil
}

func (s3fs *fakes3fsMounter) Unstage(stageTarget string) error {
	klog.Infof("-S3FSMounter Unstage-")
	return nil
}

func (s3fs *fakes3fsMounter) Mount(source string, target string) error {
	return nil
}

func (s3fs *fakes3fsMounter) Unmount(target string) error {
	return nil
}
