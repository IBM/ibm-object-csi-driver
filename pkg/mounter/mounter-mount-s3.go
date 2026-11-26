/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2026 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

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

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"k8s.io/klog/v2"
)

type MountS3Mounter struct {
	BucketName    string
	ObjectPath    string
	EndPoint      string
	LocConstraint string
	AuthType      string
	AccessKey     string
	SecretKey     string

	UID string
	GID string

	LogLevel string
	CacheDir string

	ReadOnly bool

	// Deduplicated passthrough flags; secret flags override SC flags.
	MountOptions []string
	MounterUtils utils.MounterUtils
}

const (
	s3MountLogDirectory = "/var/log/mount-s3"

	s3MountCredentialsFile = "credentials"
	s3MountConfigFile      = "config"
	s3MountAwsProfile      = "default"
)

var (
	removeS3ConfigFile = removeConfigDir

	// User-supplied flags that are hardcoded, driver-managed or unsupported, with the reason.
	s3MountIgnoredOptions = map[string]string{
		"allow-other":      "is always enabled and cannot be overridden",
		"umask":            "is not supported by mount-s3, use --dir-mode / --file-mode instead",
		"log-directory":    fmt.Sprintf("is hardcoded to %q and cannot be overridden", s3MountLogDirectory),
		"force-path-style": "is hardcoded (required for IBM COS) and cannot be overridden",
		"read-only":        "is set via MounterParams and cannot be overridden via mount options",
		"allow-delete":     "is hardcoded (enabled unless read-only) and cannot be overridden",
		"allow-overwrite":  "is hardcoded (enabled unless read-only) and cannot be overridden",
		"incremental-upload": "requires AWS S3 Express One Zone and causes data loss on IBM COS",
	}
)

// s3MounterArgs must stay in sync with s3MounterArgs in cos-csi-mounter.
type s3MounterArgs struct {
	AllowOther         string `json:"allow-other,omitempty"`
	AwsCredentialsFile string `json:"aws-credentials-file,omitempty"`
	AwsConfigFile      string `json:"aws-config-file,omitempty"`

	EndpointURL string `json:"endpoint-url,omitempty"`
	Region      string `json:"region,omitempty"`

	UID string `json:"uid,omitempty"`
	GID string `json:"gid,omitempty"`

	LogLevel     string `json:"log-level,omitempty"`
	LogDirectory string `json:"log-directory,omitempty"`

	CacheDir string `json:"cache,omitempty"`

	ReadOnly       string `json:"read-only,omitempty"`
	AllowDelete    string `json:"allow-delete,omitempty"`
	AllowOverwrite string `json:"allow-overwrite,omitempty"`

	ForcePathStyle string `json:"force-path-style,omitempty"`

	// Passthrough flags not handled by the fields above.
	Args []string `json:"args,omitempty"`
}

type MountS3MounterParams struct {
	SecretMap    map[string]string
	MountOptions []string
	MounterUtils utils.MounterUtils
	Gid          string
	ReadOnly     bool
}

// NewMountS3Mounter creates a new MountS3Mounter from the unified
// secret format.
//
// Precedence (lowest → highest):
//  1. secretMap identity keys
//  2. StorageClass mountOptions — parsed first
//  3. Secret mountOptions — parsed second, wins over SC on any overlapping flag
//
// For passthrough flags, deduplication is applied: if the same flag key appears
// in both SC and secret, the SC entry is dropped and the secret entry is kept.
func NewMountS3Mounter(params MountS3MounterParams) Mounter {
	secretMap := params.SecretMap
	mountOptions := params.MountOptions
	mounterUtils := params.MounterUtils
	gid := params.Gid
	readOnly := params.ReadOnly

	klog.Info("-newMountS3Mounter-")

	mounter := &MountS3Mounter{}
	mounter.MounterUtils = mounterUtils

	if val, ok := secretMap["accessKey"]; ok {
		mounter.AccessKey = val
	}
	if val, ok := secretMap["secretKey"]; ok {
		mounter.SecretKey = val
	}
	if val, ok := secretMap["bucketName"]; ok {
		mounter.BucketName = val
	}
	if val, ok := secretMap["objectPath"]; ok {
		mounter.ObjectPath = val
	}
	if val, ok := secretMap["cosEndpoint"]; ok {
		mounter.EndPoint = val
	}
	if val, ok := secretMap["locationConstraint"]; ok {
		mounter.LocConstraint = val
	}

	if gid != "" && secretMap["uid"] == "" {
		mounter.UID = gid
	} else if val := secretMap["uid"]; val != "" {
		mounter.UID = val
	}
	if val := secretMap["gid"]; val != "" {
		mounter.GID = val
	} else {
		mounter.GID = gid
	}

	mounter.ReadOnly = readOnly

	mounter.AuthType = "hmac"

	// Step 1: parse SC options.
	scRemainder := parseMountS3Options(mounter, mountOptions)

	// Step 2: parse secret options (structured fields set by secret override SC).
	secretOpts := splitSecretMountOptions(secretMap["mountOptions"])
	secretRemainder := parseMountS3Options(mounter, secretOpts)

	// Step 3: deduplicate passthroughs — secret wins over SC on same key.
	mounter.MountOptions = deduplicatePassthrough(scRemainder, secretRemainder)

	klog.Infof("newMountS3Mounter args:\n\tbucketName: [%s]\n\tobjectPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.BucketName, mounter.ObjectPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType)
	klog.Infof("newMountS3Mounter: SC mountOptions=%v secretMountOptions=%v deduplicated passthrough=%v",
		mountOptions, secretOpts, mounter.MountOptions)

	return mounter
}

// deduplicatePassthrough merges SC and secret passthrough flags.
// If the same flag key appears in both, the SC entry is dropped and the
// secret entry is kept (secret wins).
//
// Examples:
//
//	SC:     [--max-threads=32, --maximum-throughput-gbps=10, --allow-delete]
//	Secret: [--max-threads=128, --maximum-throughput-gbps=25]
//	Result: [--allow-delete, --max-threads=128, --maximum-throughput-gbps=25]
func deduplicatePassthrough(scFlags, secretFlags []string) []string {
	// flagKey extracts the key portion of a flag:
	// "--max-threads=128" → "max-threads", "--allow-delete" → "allow-delete".
	flagKey := func(flag string) string {
		key, _, _ := strings.Cut(strings.TrimPrefix(flag, "--"), "=")
		return key
	}

	// Build a set of keys present in secret flags.
	secretKeys := make(map[string]struct{}, len(secretFlags))
	for _, flag := range secretFlags {
		secretKeys[flagKey(flag)] = struct{}{}
	}

	// Keep SC flags whose key does NOT appear in secret flags, then append
	// all secret flags last.
	result := make([]string, 0, len(scFlags)+len(secretFlags))
	for _, flag := range scFlags {
		if _, overridden := secretKeys[flagKey(flag)]; overridden {
			klog.Infof("deduplicatePassthrough: SC flag %q overridden by secret", flag)
			continue
		}
		result = append(result, flag)
	}
	return append(result, secretFlags...)
}

// splitSecretMountOptions splits the multiline mountOptions string from the secret.
func splitSecretMountOptions(raw string) []string {
	if raw == "" {
		return nil
	}
	var opts []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimRight(line, ",")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		opts = append(opts, line)
	}
	return opts
}

// parseMountS3Options parses mount-s3 options from the provided slice.
func parseMountS3Options(mounter *MountS3Mounter, opts []string) []string {
	var remaining []string

	for _, opt := range opts {
		trimmed := strings.TrimPrefix(opt, "--")
		key, value, hasValue := strings.Cut(trimmed, "=")

		if reason, ignored := s3MountIgnoredOptions[key]; ignored {
			klog.Warningf("parseMountS3Options: '%s' %s. Ignoring.", key, reason)
			continue
		}

		switch key {
		case "uid":
			if hasValue {
				mounter.UID = value
			}
		case "gid":
			if hasValue {
				mounter.GID = value
			}
		case "log-level":
			if hasValue {
				mounter.LogLevel = value
			}
		case "cache":
			if hasValue {
				mounter.CacheDir = value
			}
		default:
			if strings.HasPrefix(opt, "--") {
				remaining = append(remaining, opt)
			} else {
				remaining = append(remaining, "--"+opt)
			}
		}
	}

	return remaining
}

func (s3 *MountS3Mounter) Mount(source string, target string) error {
	klog.Info("-MountS3Mounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

	var configPath string
	if mountWorker {
		configPath = constants.MounterConfigPathOnHost
	} else {
		configPath = constants.MounterConfigPathOnPodS3Mount
	}

	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if err := createS3MountConfig(configPathWithVolID, s3); err != nil {
		klog.Errorf("MountS3Mounter Mount: Cannot create config file: %v", err)
		return err
	}

	bucketName := s3.BucketName
	if s3.ObjectPath != "" {
		trimmedPath := strings.TrimPrefix(s3.ObjectPath, "/")
		bucketName = fmt.Sprintf("%s/%s", s3.BucketName, trimmedPath)
	}

	args, envVars, wnOp := s3.formulateMountOptions(bucketName, target, configPathWithVolID)

	if mountWorker {
		klog.Info("Mount on Worker started...")

		mountReq := struct {
			Path    string        `json:"path"`
			Bucket  string        `json:"bucket"`
			Mounter string        `json:"mounter"`
			Args    s3MounterArgs `json:"args"`
		}{
			Path:    target,
			Bucket:  bucketName,
			Mounter: constants.AMAZONS3MOUNTER,
			Args:    wnOp,
		}
		payloadBytes, err := json.Marshal(mountReq)
		if err != nil {
			klog.Errorf("Error marshalling mount request: %v", err)
			return err
		}

		klog.Infof("MountS3Mounter: worker payload: %s", payloadBytes)

		err = mounterRequest(string(payloadBytes), "http://unix/api/cos/mount")
		if err != nil {
			klog.Errorf("failed to mount on worker: %v", err)
			return err
		}
		return nil
	}

	klog.Info("NodeServer Mounting...")

	return s3.MounterUtils.FuseMountWithEnv(target, constants.MountS3BinaryPath, args, envVars)
}

// Unmount unmounts the S3 bucket.
func (s3 *MountS3Mounter) Unmount(target string) error {
	klog.Info("-MountS3Mounter Unmount-")

	if mountWorker {
		klog.Info("Unmount on Worker started...")

		unmountReq := struct {
			Path string `json:"path"`
		}{
			Path: target,
		}
		payloadBytes, err := json.Marshal(unmountReq)
		if err != nil {
			klog.Errorf("Error marshalling unmount request: %v", err)
			return err
		}

		err = mounterRequest(string(payloadBytes), "http://unix/api/cos/unmount")
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

// createS3MountConfig creates the AWS credentials and config files.
func createS3MountConfig(configPathWithVolID string, s3 *MountS3Mounter) error {
	if err := MakeDir(configPathWithVolID, 0755); err != nil { // #nosec G301 -- config directory needs to be readable by mount-s3 process
		klog.Errorf("MountS3Mounter: Cannot create config dir %s: %v", configPathWithVolID, err)
		return err
	}

	credFile := path.Join(configPathWithVolID, s3MountCredentialsFile)
	credParams := []string{
		"[" + s3MountAwsProfile + "]",
		"aws_access_key_id = " + s3.AccessKey,
		"aws_secret_access_key = " + s3.SecretKey,
	}
	if err := writeConfigFile(credFile, credParams); err != nil {
		klog.Errorf("MountS3Mounter: Cannot write credentials file: %v", err)
		return err
	}

	region := s3.LocConstraint
	configParams := []string{
		"[" + s3MountAwsProfile + "]",
		"region = " + region,
	}
	if s3.EndPoint != "" {
		configParams = append(configParams, "endpoint_url = "+s3.EndPoint)
	}
	cfgFile := path.Join(configPathWithVolID, s3MountConfigFile)
	if err := writeConfigFile(cfgFile, configParams); err != nil {
		klog.Errorf("MountS3Mounter: Cannot write config file: %v", err)
		return err
	}

	klog.Infof("MountS3Mounter: Created config files at %s", configPathWithVolID)
	return nil
}

// writeConfigFile writes lines to a file with 0600 permissions.
func writeConfigFile(filePath string, lines []string) error {
	f, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600) // #nosec G304 -- file path is constructed from controlled configPathWithVolID and constant filename
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			klog.Errorf("writeConfigFile: cannot close file %s: %v", filePath, cerr)
		}
	}()

	w := bufio.NewWriter(f)
	for _, line := range lines {
		if _, err := w.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("cannot write to file %s: %w", filePath, err)
		}
	}
	return w.Flush()
}

// formulateMountOptions builds the CLI args, env vars, and worker payload for mount-s3.
func (s3 *MountS3Mounter) formulateMountOptions(bucket, target, configPathWithVolID string) (nodeServerOp []string, envVars []string, workerNodeOp s3MounterArgs) {
	nodeServerOp = []string{bucket, target, "--allow-other"}

	credFilePath := path.Join(configPathWithVolID, s3MountCredentialsFile)
	cfgFilePath := path.Join(configPathWithVolID, s3MountConfigFile)
	envVars = []string{
		"AWS_SHARED_CREDENTIALS_FILE=" + credFilePath,
		"AWS_CONFIG_FILE=" + cfgFilePath,
	}

	workerNodeOp = s3MounterArgs{
		AllowOther:         "true",
		AwsCredentialsFile: credFilePath,
		AwsConfigFile:      cfgFilePath,
	}

	if s3.EndPoint != "" {
		nodeServerOp = append(nodeServerOp, "--endpoint-url="+s3.EndPoint)
		workerNodeOp.EndpointURL = s3.EndPoint
	}
	if s3.LocConstraint != "" {
		nodeServerOp = append(nodeServerOp, "--region="+s3.LocConstraint)
		workerNodeOp.Region = s3.LocConstraint
	}
	if s3.UID != "" {
		nodeServerOp = append(nodeServerOp, "--uid="+s3.UID)
		workerNodeOp.UID = s3.UID
	}
	if s3.GID != "" {
		nodeServerOp = append(nodeServerOp, "--gid="+s3.GID)
		workerNodeOp.GID = s3.GID
	}

	switch s3.LogLevel {
	case "debug":
		nodeServerOp = append(nodeServerOp, "--debug")
		workerNodeOp.LogLevel = "debug"
	case "debug-crt":
		nodeServerOp = append(nodeServerOp, "--debug-crt")
		workerNodeOp.LogLevel = "debug-crt"
	case "no-log":
		nodeServerOp = append(nodeServerOp, "--no-log")
		workerNodeOp.LogLevel = "no-log"
	default:
		if s3.LogLevel != "" {
			klog.Warningf("MountS3Mounter: unsupported log-level %q. "+
				"Supported: 'debug', 'debug-crt', 'no-log'. Ignoring.", s3.LogLevel)
		}
	}

	nodeServerOp = append(nodeServerOp, "--log-directory="+s3MountLogDirectory)
	workerNodeOp.LogDirectory = s3MountLogDirectory

	if s3.CacheDir != "" {
		nodeServerOp = append(nodeServerOp, "--cache="+s3.CacheDir)
		workerNodeOp.CacheDir = s3.CacheDir
	}

	nodeServerOp = append(nodeServerOp, "--force-path-style")
	workerNodeOp.ForcePathStyle = "true"

	// read-only suppresses allow-delete and allow-overwrite.
	if s3.ReadOnly {
		nodeServerOp = append(nodeServerOp, "--read-only")
		workerNodeOp.ReadOnly = "true"
	} else {
		nodeServerOp = append(nodeServerOp, "--allow-delete", "--allow-overwrite")
		workerNodeOp.AllowDelete = "true"
		workerNodeOp.AllowOverwrite = "true"
	}

	nodeServerOp = append(nodeServerOp, s3.MountOptions...)
	workerNodeOp.Args = s3.MountOptions

	klog.Infof("MountS3 nodeServerOp: %v", nodeServerOp)
	klog.Infof("MountS3 workerNodeOp: %+v", workerNodeOp)

	return nodeServerOp, envVars, workerNodeOp
}
