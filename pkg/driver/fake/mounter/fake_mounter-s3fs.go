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
	"github.com/IBM/satellite-object-storage-plugin/pkg/mounter"
	"github.com/golang/glog"
)

// Mounter interface defined in mounter.go
// s3fsMounter Implements Mounter
type fakes3fsMounter struct {
	bucketName string //From Secret in SC
	objPath    string //From Secret in SC
	endPoint   string //From Secret in SC
	regnClass  string //From Secret in SC
	accessKeys string
}

func fakenewS3fsMounter(bucket string, objpath string, endpoint string, region string, keys string) (mounter.Mounter, error) {
	glog.Infof("-newS3fsMounter-")
	glog.Infof("newS3fsMounter args:\n\tbucket: <%s>\n\tobjpath: <%s>\n\tendpoint: <%s>\n\tregion: <%s>\n\tkeys: <%s>", bucket, objpath, endpoint, region, keys)
	return &fakes3fsMounter{
		bucketName: bucket,
		objPath:    objpath,
		endPoint:   endpoint,
		regnClass:  region,
		accessKeys: keys,
	}, nil
}

func (s3fs *fakes3fsMounter) Stage(stageTarget string) error {
	glog.Infof("-S3FSMounter Stage-")
	return nil
}

func (s3fs *fakes3fsMounter) Unstage(stageTarget string) error {
	glog.Infof("-S3FSMounter Unstage-")
	return nil
}

func (s3fs *fakes3fsMounter) Mount(source string, target string) error {
	return nil
}

func (s3fs *fakes3fsMounter) Unmount(target string) error {
	return nil
}
