// pkg/mounter/mounter-mountpoint-s3.go
package mounter

import (
	"encoding/json"
	"fmt"
	"strings"
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

// type MountpointS3Mounter struct {
// 	Bucket       string
// 	Endpoint     string
// 	Region       string
// 	Prefix       string
// 	ReadOnly     bool
// 	UID          string
// 	GID          string
// 	MounterUtils utils.MounterUtils
// }

type MountpointS3Mounter struct {
	BucketName   string // from secret / SC
	ObjPath      string // optional prefix
	Region       string
	Endpoint     string
	ReadOnly     bool
	UID          string
	GID          string
	MountOptions []string

	MounterUtils utils.MounterUtils
}
// const (
// 	passFile   = ".passwd-s3fs" // #nosec G101: not password
// 	maxRetries = 3
// )

// var (
// 	writePassWrap = writePass
// 	removeFile    = removeS3FSCredFile
// )


// func NewMountpointS3Mounter(
// 	secretMap map[string]string,
// 	mountFlags []string,
// 	mounterUtils utils.MounterUtils,
// ) Mounter {

// 	return &MountpointS3Mounter{
// 		Bucket:   secretMap["bucketName"],
// 		Endpoint: secretMap["endpoint"],
// 		Region:  secretMap["region"],
// 		Prefix:  secretMap["prefix"],
// 		UID:     secretMap["uid"],
// 		GID:     secretMap["gid"],
// 		MounterUtils: mounterUtils,
// 	}
// }
// func (m *MountpointS3Mounter) Mount(source, target string) error {
// 	klog.Infof("MountpointS3 Mount target=%s", target)

// 	args := map[string]string{
// 		"endpoint": m.Endpoint,
// 		"region":   m.Region,
// 		"uid":      m.UID,
// 		"gid":      m.GID,
// 	}

// 	if m.Prefix != "" {
// 		args["prefix"] = m.Prefix
// 	}

// 	payloadMap := map[string]interface{}{
// 		"path":    target,
// 		"bucket":  m.Bucket,
// 		"mounter": constants.MountpointS3,
// 		"args":    args,
// 	}

// 	payload, _ := json.Marshal(payloadMap)

// 	return mounterRequest(
// 		string(payload),
// 		"http://unix/api/cos/mount",
// 	)
// }
// func (m *MountpointS3Mounter) Unmount(target string) error {
// 	klog.Infof("MountpointS3 Unmount target=%s", target)

// 	payload := fmt.Sprintf(`{"path":"%s"}`, target)

// 	return mounterRequest(
// 		payload,
// 		"http://unix/api/cos/unmount",
// 	)
// }

func NewMountpointS3Mounter(
	secretMap map[string]string,
	mountOptions []string,
	mounterUtils utils.MounterUtils,
) Mounter {

	m := &MountpointS3Mounter{}

	if v, ok := secretMap["bucketName"]; ok {
		m.BucketName = v
	}
	if v, ok := secretMap["objPath"]; ok {
		m.ObjPath = v
	}
	if v, ok := secretMap["region"]; ok {
		m.Region = v
	}
	if v, ok := secretMap["endpoint"]; ok {
		m.Endpoint = v
	}
	if v, ok := secretMap["uid"]; ok {
		m.UID = v
	}
	if v, ok := secretMap["gid"]; ok {
		m.GID = v
	}
	if v, ok := secretMap["readOnly"]; ok && v == "true" {
		m.ReadOnly = true
	}

	m.MountOptions = mountOptions
	m.MounterUtils = mounterUtils

	klog.Infof(
		"newMountpointS3Mounter bucket=%s prefix=%s region=%s endpoint=%s readonly=%v",
		m.BucketName, m.ObjPath, m.Region, m.Endpoint, m.ReadOnly,
	)

	return m
}
func (m *MountpointS3Mounter) Mount(source, target string) error {
	klog.Info("-MountpointS3 Mount-")
	klog.Infof("Mount target: %s", target)

	bucket := m.BucketName
	if m.ObjPath != "" {
		bucket = fmt.Sprintf("%s:%s", bucket, strings.TrimPrefix(m.ObjPath, "/"))
	}

	// Arguments sent to mounter daemon
	args := map[string]string{
		"region": m.Region,
	}

	if m.Endpoint != "" {
		args["endpoint-url"] = m.Endpoint
	}
	if m.ReadOnly {
		args["read-only"] = "true"
	}
	if m.UID != "" {
		args["uid"] = m.UID
	}
	if m.GID != "" {
		args["gid"] = m.GID
	}

	for _, opt := range m.MountOptions {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) == 2 {
			args[parts[0]] = parts[1]
		} else {
			args[parts[0]] = "true"
		}
	}

	payload := map[string]interface{}{
		"path":    target,
		"bucket":  bucket,
		"mounter": constants.MountpointS3,
		"args":    args,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	klog.Infof("Worker Mounting Payload: %s", string(data))

	return mounterRequest(
		string(data),
		"http://unix/api/cos/mount",
	)
}
func (m *MountpointS3Mounter) Unmount(target string) error {
	klog.Info("-MountpointS3 Unmount-")
	klog.Infof("Unmount target: %s", target)

	payload := fmt.Sprintf(`{"path":"%s"}`, target)

	return mounterRequest(
		payload,
		"http://unix/api/cos/unmount",
	)
}
