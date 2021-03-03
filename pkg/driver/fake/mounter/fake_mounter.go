/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

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
