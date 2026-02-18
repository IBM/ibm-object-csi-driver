// Package mounter
package mounter

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

// MountpointS3Mounter implements Mounter for AWS Mountpoint S3
type MountpointS3Mounter struct {
	BucketName    string
	ObjectPath    string
	EndPoint      string
	LocConstraint string
	AuthType      string
	AccessKey     string
	SecretKey     string
	UID           string
	GID           string
	UMask         string
	LogLevel      string
	ReadOnly      bool
	MountOptions  []string
	MounterUtils  utils.MounterUtils
}

// s3MounterArgs holds the args passed to the worker for mount-s3
type s3MounterArgs struct {
	ReadOnly   string `json:"read-only,omitempty"`
	AllowOther string `json:"allow-other,omitempty"`
	UID        string `json:"uid,omitempty"`
	GID        string `json:"gid,omitempty"`
	UMask      string `json:"umask,omitempty"`
	LogLevel   string `json:"log-level,omitempty"`
	// Config file path for AWS credentials
	AwsConfigDir string `json:"aws-config-dir,omitempty"`
	EndpointURL  string `json:"endpoint-url,omitempty"`
	Region       string `json:"region,omitempty"`
}

const (
	s3MountCredentialsFile = "credentials"
	s3MountConfigFile      = "config"
	s3MountAwsProfile      = "default"
)

var (
	removeS3ConfigFile = removeS3MountConfigFile
)

// NewMountpointS3Mounter creates a new MountpointS3Mounter from secretMap
func NewMountpointS3Mounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
	klog.Info("-newMountpointS3Mounter-")

	var (
		val     string
		check   bool
		mounter = &MountpointS3Mounter{}
	)

	if val, check = secretMap["cosEndpoint"]; check {
		mounter.EndPoint = val
	}
	if val, check = secretMap["locationConstraint"]; check {
		mounter.LocConstraint = val
	}
	if val, check = secretMap["bucketName"]; check {
		mounter.BucketName = val
	}
	if val, check = secretMap["objectPath"]; check {
		mounter.ObjectPath = val
	}
	if val, check = secretMap["accessKey"]; check {
		mounter.AccessKey = val
	}
	if val, check = secretMap["secretKey"]; check {
		mounter.SecretKey = val
	}
	if val, check = secretMap["uid"]; check {
		mounter.UID = val
	}
	// If gid set but uid not, use gid for uid (same as rclone behavior)
	if secretMap["gid"] != "" && secretMap["uid"] == "" {
		mounter.UID = secretMap["gid"]
	} else if secretMap["uid"] != "" {
		mounter.UID = secretMap["uid"]
	}
	if val, check = secretMap["gid"]; check {
		mounter.GID = val
	}
	if val, check = secretMap["umask"]; check {
		mounter.UMask = val
	}
	if val, check = secretMap["logLevel"]; check {
		mounter.LogLevel = val
	}
	if val, check = secretMap["readOnly"]; check {
		mounter.ReadOnly = val == "true"
	}

	mounter.AuthType = "hmac"

	klog.Infof("newMountpointS3Mounter args:\n\tbucketName: [%s]\n\tobjectPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.BucketName, mounter.ObjectPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType)

	mounter.MountOptions = mountOptions
	mounter.MounterUtils = mounterUtils

	return mounter
}

// Mount mounts the S3 bucket using mountpoint-s3
func (s3 *MountpointS3Mounter) Mount(source string, target string) error {
	klog.Info("-MountpointS3Mounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

	// Determine config path based on mode 
	var configPath string
	if mountWorker {
		configPath = constants.MounterConfigPathOnHost
	} else {
		configPath = constants.MounterConfigPathOnPodS3Mount
	}

	// Create unique config dir per volume 
	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))
	
	// configCreated := false

    // // Cleanup on failure
    // defer func() {
    //     if err != nil && configCreated {
    //         klog.Warningf("Mount failed, cleaning up config dir: %s", configPathWithVolID)
    //         removeS3ConfigFile(configPath, target)
    //     }
    // }()

	// Write AWS credentials config file
	if err := createS3MountConfig(configPathWithVolID, s3); err != nil {
		klog.Errorf("MountpointS3Mounter Mount: Cannot create config file: %v", err)
		return err
	}

	// configCreated = true

	// Build bucket path with optional object path prefix
	bucketName := s3.BucketName
	if s3.ObjectPath != "" {
		trimmedPath := strings.TrimPrefix(s3.ObjectPath, "/")
		bucketName = fmt.Sprintf("%s/%s", s3.BucketName, trimmedPath)
	}

	args, wnOp := s3.formulateMountOptions(bucketName, target, configPathWithVolID)

	if mountWorker {
		klog.Info("Mount on Worker started...")

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			klog.Errorf("Error marshalling mount args: %v", err)
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`,
			target, bucketName, constants.MountpointS3, jsonData)

		err = mounterRequest(payload, "http://unix/api/cos/mount")
		if err != nil {
			klog.Errorf("failed to mount on worker: %v", err)
			return err
		}
		return nil
	}

	klog.Info("NodeServer Mounting...")
	return s3.MounterUtils.FuseMount(target, constants.MountpointS3BinaryPath, args)
}

// Unmount unmounts the S3 bucket
func (s3 *MountpointS3Mounter) Unmount(target string) error {
	klog.Info("-MountpointS3Mounter Unmount-")

	if mountWorker {
		klog.Info("Unmount on Worker started...")

		payload := fmt.Sprintf(`{"path":"%s"}`, target)

		err := mounterRequest(payload, "http://unix/api/cos/unmount")
		if err != nil {
			klog.Errorf("failed to unmount on worker: %v", err)
			return err
		}

		removeS3ConfigFile(constants.MounterConfigPathOnHost, target)
		return nil
	}

	klog.Info("NodeServer Unmounting...")
	err := s3.MounterUtils.FuseUnmount(target)
	if err != nil {
		return err
	}

	removeS3ConfigFile(constants.MounterConfigPathOnPodS3Mount, target)
	return nil
}

// createS3MountConfig creates the AWS credentials and config files
// that mount-s3 reads via --aws-config-dir flag.
// Directory structure:
//
//	<configPathWithVolID>/
//	  credentials   <- AWS credentials (access/secret key)
//	  config        <- AWS config (region, endpoint)
func createS3MountConfig(configPathWithVolID string, s3 *MountpointS3Mounter) error {
	// Create the config directory
	if err := MakeDir(configPathWithVolID, 0755); err != nil { // #nosec G301
		klog.Errorf("MountpointS3Mounter: Cannot create config dir %s: %v", configPathWithVolID, err)
		return err
	}

	// --- Write credentials file ---
	// Format matches AWS credentials file:
	// [default]
	// aws_access_key_id = <key>
	// aws_secret_access_key = <secret>
	credFile := path.Join(configPathWithVolID, s3MountCredentialsFile)
	credParams := []string{
		"[" + s3MountAwsProfile + "]",
		"aws_access_key_id = " + s3.AccessKey,
		"aws_secret_access_key = " + s3.SecretKey,
	}
	if err := writeConfigFile(credFile, credParams); err != nil {
		klog.Errorf("MountpointS3Mounter: Cannot write credentials file: %v", err)
		return err
	}

	// --- Write config file ---
	// Format matches AWS config file:
	// [default]
	// region = <region>
	// endpoint_url = <endpoint>   (for IBM COS compatibility)
	region := convertLocationConstraintToRegion(s3.LocConstraint)
	configParams := []string{
		"[" + s3MountAwsProfile + "]",
		"region = " + region,
	}
	if s3.EndPoint != "" {
		configParams = append(configParams, "endpoint_url = "+s3.EndPoint)
	}
	cfgFile := path.Join(configPathWithVolID, s3MountConfigFile)
	if err := writeConfigFile(cfgFile, configParams); err != nil {
		klog.Errorf("MountpointS3Mounter: Cannot write config file: %v", err)
		return err
	}

	klog.Infof("MountpointS3Mounter: Created config files at %s", configPathWithVolID)
	return nil
}

// writeConfigFile writes lines to a file with 0600 permissions
func writeConfigFile(filePath string, lines []string) error {
	f, err := CreateFile(filePath) // #nosec G304
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			klog.Errorf("writeConfigFile: cannot close file %s: %v", filePath, cerr)
		}
	}()

	if err := Chmod(filePath, 0600); err != nil { // #nosec G302
		return fmt.Errorf("cannot chmod file %s: %w", filePath, err)
	}

	w := bufio.NewWriter(f)
	for _, line := range lines {
		if _, err := w.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("cannot write to file %s: %w", filePath, err)
		}
	}
	return w.Flush()
}

// formulateMountOptions builds CLI args for mount-s3 and the worker payload
func (s3 *MountpointS3Mounter) formulateMountOptions(bucket, target, configPathWithVolID string) (nodeServerOp []string, workerNodeOp s3MounterArgs) {
	// mount-s3 syntax: mount-s3 <bucket> <mountpoint> [flags]
	nodeServerOp = []string{
		bucket,
		target,
		"--allow-other",
		// Point mount-s3 to our config directory for credentials & config
		"--aws-config-dir=" + configPathWithVolID,
	}

	workerNodeOp = s3MounterArgs{
		AllowOther:   "true",
		AwsConfigDir: configPathWithVolID,
	}

	// Endpoint (IBM COS endpoint)
	if s3.EndPoint != "" {
		nodeServerOp = append(nodeServerOp, "--endpoint-url="+s3.EndPoint)
		workerNodeOp.EndpointURL = s3.EndPoint
	}

	// Region from location constraint
	// Not rquired because already passed in config file, There is no issue in passing it with CLI but may create an issue if the regions are differnet..
	// if s3.LocConstraint != "" {
	// 	region := convertLocationConstraintToRegion(s3.LocConstraint)
	// 	nodeServerOp = append(nodeServerOp, "--region="+region)
	// 	workerNodeOp.Region = region
	// }

	// UID
	if s3.UID != "" {
		nodeServerOp = append(nodeServerOp, "--uid="+s3.UID)
		workerNodeOp.UID = s3.UID
	}

	// GID
	if s3.GID != "" {
		nodeServerOp = append(nodeServerOp, "--gid="+s3.GID)
		workerNodeOp.GID = s3.GID
	}

	// UMask
	if s3.UMask != "" {
		nodeServerOp = append(nodeServerOp, "--umask="+s3.UMask)
		workerNodeOp.UMask = s3.UMask
	}

	// Log level
	logLevel := s3.LogLevel
	if logLevel == "" {
		logLevel = "warn"
	}
	nodeServerOp = append(nodeServerOp, "--log-level="+logLevel)
	workerNodeOp.LogLevel = logLevel

	// Read-only
	if s3.ReadOnly {
		nodeServerOp = append(nodeServerOp, "--read-only")
		workerNodeOp.ReadOnly = "true"
	}

	// Extra mount options from StorageClass
	nodeServerOp = append(nodeServerOp, s3.MountOptions...)

	klog.Infof("MountpointS3 nodeServerOp: %v", nodeServerOp)
	klog.Infof("MountpointS3 workerNodeOp: %+v", workerNodeOp)

	return nodeServerOp, workerNodeOp
}

// removeS3MountConfigFile removes the config directory for the volume (same pattern as rclone)
func removeS3MountConfigFile(configPath, target string) {
	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	for retry := 1; retry <= maxRetries; retry++ {
		_, err := Stat(configPathWithVolID)
		if err != nil {
			if os.IsNotExist(err) {
				klog.Infof("removeS3MountConfigFile: Config dir does not exist: %s", configPathWithVolID)
				return
			}
			klog.Errorf("removeS3MountConfigFile: Attempt %d - Failed to stat path %s: %v", retry, configPathWithVolID, err)
			time.Sleep(constants.Interval)
			continue
		}
		if err = RemoveAll(configPathWithVolID); err != nil {
			klog.Errorf("removeS3MountConfigFile: Attempt %d - Failed to remove config dir %s: %v", retry, configPathWithVolID, err)
			time.Sleep(constants.Interval)
			continue
		}
		klog.Infof("removeS3MountConfigFile: Successfully removed config dir: %s", configPathWithVolID)
		return
	}
	klog.Errorf("removeS3MountConfigFile: Failed to remove config dir after %d attempts", maxRetries)
}

// convertLocationConstraintToRegion converts IBM COS location constraints
// to AWS-style region strings that mountpoint-s3 understands
// e.g. "us-east-standard" -> "us-east-1"
func convertLocationConstraintToRegion(locationConstraint string) string {
	tiers := []string{"-standard", "-smart", "-cold", "-flex", "-vault"}
	region := locationConstraint
	for _, tier := range tiers {
		region = strings.TrimSuffix(region, tier)
	}

	regionMap := map[string]string{
		"us-east":  "us-east-1",
		"us-south": "us-south",
		"eu-de":    "eu-de",
		"eu-gb":    "eu-gb",
		"ap-north": "ap-north",
		"ap-south": "ap-south",
		"jp-tok":   "jp-tok",
		"au-syd":   "au-syd",
		"ca-tor":   "ca-tor",
		"br-sao":   "br-sao",
		"global":   "global",
	}

	if mapped, ok := regionMap[region]; ok {
		return mapped
	}

	klog.Warningf("No region mapping found for location constraint: %s, using as-is: %s", locationConstraint, region)
	return region
}