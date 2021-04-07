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
	"fmt"
	"github.com/IBM/satellite-object-storage-plugin/pkg/mounter"
)

const (
	s3fsMounterType     = "s3fs"
	goofysMounterType   = "goofys"
	s3qlMounterType     = "s3ql"
	s3backerMounterType = "s3backer"
	mounterTypeKey      = "mounter"
)

//func newS3fsMounter(bucket string, objpath string, endpoint string, region string, keys string)
func NewMounter(mounter string, bucket string, objpath string, endpoint string, region string, keys string) (mounter.Mounter, error) {
	fmt.Sprint("-NewMounter-")
	fmt.Sprintf("NewMounter args:\n\tmounter: <%s>\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>", mounter, bucket, objpath, endpoint, region)
	switch mounter {
	case s3fsMounterType:
		return fakenewS3fsMounter(bucket, objpath, endpoint, region, keys)
	default:
		// default to s3backer
		return fakenewS3fsMounter(bucket, objpath, endpoint, region, keys)
	}
}
