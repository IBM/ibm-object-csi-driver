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

// MountpointS3Mounter implements Mounter for AWS Mountpoint S3.
//
// Only flags that require code-level handling are modelled as struct fields:
//   - Conflict resolution : ReadOnly takes priority — clears AllowOverwrite / IncrementalUpload
//   - Directory creation  : LogDirectory / CacheDir are created by the worker's Validate()
//   - Name translation   : LogLevel ("debug" → --debug, not --log-level=debug)
//   - Special precedence : UID / GID (secretMap < mountOptions)
//   - Always-emitted     : allow-other (deduplicated here)
//   - Invalid flag       : umask (warn + drop)
//   - SecretMap-sourced  : EndPoint / LocConstraint (injected from secret fields)
//
// All other flags are the user's responsibility and pass through unchanged to
// mount-s3 via MountOptions. The parser ensures every passthrough entry has a
// "--" prefix so mount-s3 always receives syntactically valid flags.
type MountpointS3Mounter struct {
	// Identity — sourced from secretMap, not mountOptions.
	BucketName    string
	ObjectPath    string
	EndPoint      string
	LocConstraint string
	AuthType      string
	AccessKey     string
	SecretKey     string

	// UID / GID — secretMap sets initial value; mountOptions can override.
	UID string
	GID string

	// LogLevel requires name translation:
	//   "debug"     → --debug
	//   "debug-crt" → --debug-crt
	//   "no-log"    → --no-log
	// mount-s3 has no --log-level flag.
	LogLevel string

	// LogDirectory is forwarded to the worker; worker's Validate() creates it via ensureDir.
	LogDirectory string

	// CacheDir is forwarded to the worker; worker's Validate() creates it via ensureDir.
	CacheDir string

	// Conflict resolution — ReadOnly takes priority over write flags.
	// If ReadOnly is set, AllowOverwrite and IncrementalUpload are cleared
	// in Mount() before formulateMountOptions is called. This allows SC to
	// set allow-overwrite and incremental-upload as safe defaults while still
	// letting users request read-only via their secret without errors.
	ReadOnly          bool // --read-only
	AllowOverwrite    bool // --allow-overwrite
	IncrementalUpload bool // --incremental-upload

	// MountOptions holds all remaining flags that pass through unchanged.
	// The parser ensures every entry has a "--" prefix so mount-s3 sees valid flags.
	MountOptions []string
	MounterUtils utils.MounterUtils
}

// s3MounterArgs holds the args sent to the worker node daemon for mount-s3.
// Structured fields cover all named flags; Args carries any remaining
// passthrough flags (e.g. --allow-delete, --hello, --never=true).
// This struct must stay in sync with s3MounterArgs in the worker (cos-csi-mounter).
type s3MounterArgs struct {
	// Always set.
	AllowOther         string `json:"allow-other,omitempty"`
	AwsCredentialsFile string `json:"aws-credentials-file,omitempty"`
	AwsConfigFile      string `json:"aws-config-file,omitempty"`

	// SecretMap-sourced.
	EndpointURL string `json:"endpoint-url,omitempty"`
	Region      string `json:"region,omitempty"`

	// Identity.
	UID string `json:"uid,omitempty"`
	GID string `json:"gid,omitempty"`

	// Logging (translated from LogLevel).
	LogLevel     string `json:"log-level,omitempty"`
	LogDirectory string `json:"log-directory,omitempty"`

	// Cache.
	CacheDir string `json:"cache,omitempty"`

	// Conflict trio.
	ReadOnly          string `json:"read-only,omitempty"`
	AllowOverwrite    string `json:"allow-overwrite,omitempty"`
	IncrementalUpload string `json:"incremental-upload,omitempty"`

	// Passthrough flags — user-supplied flags not handled by structured fields
	// (e.g. --allow-delete, --hello, --never=true).
	// Worker's PopulateArgsSlice appends these last so they have highest precedence.
	Args []string `json:"args,omitempty"`
}

const (
	s3MountCredentialsFile = "credentials"
	s3MountConfigFile      = "config"
	s3MountAwsProfile      = "default"
)

var (
	removeS3ConfigFile = removeS3MountConfigFile
)

// envMounter is a local interface for type assertion — allows calling
// FuseMountWithEnv on the concrete MounterOptsUtils without changing the
// MounterUtils interface, so other mounters (rclone etc.) are unaffected.
type envMounter interface {
	FuseMountWithEnv(path string, comm string, args []string, envVars []string) error
}

// NewMountpointS3Mounter creates a new MountpointS3Mounter from the unified
// secret format.
func NewMountpointS3Mounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
	klog.Info("-newMountpointS3Mounter-")

	mounter := &MountpointS3Mounter{}

	// --- Credentials ---
	if val, ok := secretMap["accessKey"]; ok {
		mounter.AccessKey = val
	}
	if val, ok := secretMap["secretKey"]; ok {
		mounter.SecretKey = val
	}

	// --- Bucket / endpoint identity ---
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

	// --- UID / GID (lowest precedence — mountOptions overrides below) ---
	if secretMap["gid"] != "" && secretMap["uid"] == "" {
		mounter.UID = secretMap["gid"]
	} else if val := secretMap["uid"]; val != "" {
		mounter.UID = val
	}
	if val := secretMap["gid"]; val != "" {
		mounter.GID = val
	}

	mounter.AuthType = "hmac"

	// --- Merge and parse mountOptions ---
	scRemainder := parseMountpointS3Options(mounter, mountOptions)
	secretOpts := splitSecretMountOptions(secretMap["mountOptions"])
	secretRemainder := parseMountpointS3Options(mounter, secretOpts)
	mounter.MountOptions = append(scRemainder, secretRemainder...)
	mounter.MounterUtils = mounterUtils

	klog.Infof("newMountpointS3Mounter args:\n\tbucketName: [%s]\n\tobjectPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.BucketName, mounter.ObjectPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType)
	klog.Infof("newMountpointS3Mounter: SC mountOptions=%v secretMountOptions=%v passthrough=%v",
		mountOptions, secretOpts, mounter.MountOptions)

	return mounter
}

// splitSecretMountOptions splits the multiline mountOptions string from the secret.
func splitSecretMountOptions(raw string) []string {
	if raw == "" {
		return nil
	}
	var opts []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		opts = append(opts, line)
	}
	return opts
}

// parseMountpointS3Options parses mount-s3 options from the provided slice.
func parseMountpointS3Options(mounter *MountpointS3Mounter, opts []string) []string {
	var remaining []string

	for _, opt := range opts {
		trimmed := strings.TrimPrefix(opt, "--")
		key, value, hasValue := strings.Cut(trimmed, "=")

		switch key {
		case "allow-other":
			// formulateMountOptions unconditionally adds --allow-other.
		case "umask":
			klog.Warningf("parseMountpointS3Options: 'umask' is not supported by mount-s3. " +
				"Use --dir-mode / --file-mode instead. Ignoring.")
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
		case "log-directory":
			if hasValue {
				mounter.LogDirectory = value
			}
		case "cache":
			if hasValue {
				mounter.CacheDir = value
			}
		case "read-only":
			mounter.ReadOnly = true
		case "allow-overwrite":
			mounter.AllowOverwrite = true
		case "incremental-upload":
			mounter.IncrementalUpload = true
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

// Mount mounts the S3 bucket using mountpoint-s3.
func (s3 *MountpointS3Mounter) Mount(source string, target string) error {
	klog.Info("-MountpointS3Mounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

	// --- Read-only priority resolution ---
	if s3.ReadOnly {
		if s3.AllowOverwrite {
			klog.Infof("MountpointS3Mounter: read-only set — clearing allow-overwrite")
			s3.AllowOverwrite = false
		}
		if s3.IncrementalUpload {
			klog.Infof("MountpointS3Mounter: read-only set — clearing incremental-upload")
			s3.IncrementalUpload = false
		}

		var filtered []string
		for _, opt := range s3.MountOptions {
			trimmed := strings.TrimPrefix(opt, "--")
			if trimmed != "allow-delete" {
				filtered = append(filtered, opt)
			} else {
				klog.Infof("MountpointS3Mounter: read-only set — removing allow-delete from passthrough options")
			}
		}
		s3.MountOptions = filtered
	}

	// NOTE: No directory validation or creation here.
	// LogDirectory and CacheDir are forwarded to the worker via workerNodeOp.
	// The worker's Validate() calls ensureDir() which creates them if missing.

	// --- Config path ---
	var configPath string
	if mountWorker {
		configPath = constants.MounterConfigPathOnHost
	} else {
		configPath = constants.MounterConfigPathOnPodS3Mount
	}

	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	if err := createS3MountConfig(configPathWithVolID, s3); err != nil {
		klog.Errorf("MountpointS3Mounter Mount: Cannot create config file: %v", err)
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

		jsonData, err := json.Marshal(wnOp)
		if err != nil {
			klog.Errorf("Error marshalling mount args: %v", err)
			return err
		}

		payload := fmt.Sprintf(`{"path":"%s","bucket":"%s","mounter":"%s","args":%s}`,
			target, bucketName, constants.AMAZONS3MOUNTER, jsonData)

		klog.Infof("MountpointS3Mounter: worker payload: %s", payload)

		err = mounterRequest(payload, "http://unix/api/cos/mount")
		if err != nil {
			klog.Errorf("failed to mount on worker: %v", err)
			return err
		}
		return nil
	}

	klog.Info("NodeServer Mounting...")

	if m, ok := s3.MounterUtils.(envMounter); ok {
		return m.FuseMountWithEnv(target, constants.MountpointS3BinaryPath, args, envVars)
	}

	klog.Warning("MounterUtils does not implement FuseMountWithEnv, falling back to FuseMount.")
	return s3.MounterUtils.FuseMount(target, constants.MountpointS3BinaryPath, args)
}

// Unmount unmounts the S3 bucket.
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

// createS3MountConfig creates the AWS credentials and config files.
func createS3MountConfig(configPathWithVolID string, s3 *MountpointS3Mounter) error {
	if err := MakeDir(configPathWithVolID, 0755); err != nil { // #nosec G301 -- config directory needs to be readable by mount-s3 process
		klog.Errorf("MountpointS3Mounter: Cannot create config dir %s: %v", configPathWithVolID, err)
		return err
	}

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
		klog.Errorf("MountpointS3Mounter: Cannot write config file: %v", err)
		return err
	}

	klog.Infof("MountpointS3Mounter: Created config files at %s", configPathWithVolID)
	return nil
}

// writeConfigFile writes lines to a file with 0600 permissions.
func writeConfigFile(filePath string, lines []string) error {
	f, err := CreateFile(filePath) // #nosec G304 -- filePath is constructed from validated volume ID and predefined config file names
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			klog.Errorf("writeConfigFile: cannot close file %s: %v", filePath, cerr)
		}
	}()

	if err := Chmod(filePath, 0600); err != nil { // #nosec G302 -- credentials file must be readable only by owner for security
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

// formulateMountOptions builds the CLI args, env vars, and worker payload for mount-s3.
func (s3 *MountpointS3Mounter) formulateMountOptions(bucket, target, configPathWithVolID string) (nodeServerOp []string, envVars []string, workerNodeOp s3MounterArgs) {
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

	// --- SecretMap-sourced ---
	if s3.EndPoint != "" {
		nodeServerOp = append(nodeServerOp, "--endpoint-url="+s3.EndPoint)
		workerNodeOp.EndpointURL = s3.EndPoint
	}
	if s3.LocConstraint != "" {
		nodeServerOp = append(nodeServerOp, "--region="+s3.LocConstraint)
		workerNodeOp.Region = s3.LocConstraint
	}

	// --- Identity ---
	if s3.UID != "" {
		nodeServerOp = append(nodeServerOp, "--uid="+s3.UID)
		workerNodeOp.UID = s3.UID
	}
	if s3.GID != "" {
		nodeServerOp = append(nodeServerOp, "--gid="+s3.GID)
		workerNodeOp.GID = s3.GID
	}

	// --- Log level (name translation) ---
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
			klog.Warningf("MountpointS3Mounter: unsupported log-level %q. "+
				"Supported: 'debug', 'debug-crt', 'no-log'. Ignoring.", s3.LogLevel)
		}
	}

	// --- Log directory ---
	if s3.LogDirectory != "" {
		nodeServerOp = append(nodeServerOp, "--log-directory="+s3.LogDirectory)
		workerNodeOp.LogDirectory = s3.LogDirectory
	}

	// --- Cache directory ---
	if s3.CacheDir != "" {
		nodeServerOp = append(nodeServerOp, "--cache="+s3.CacheDir)
		workerNodeOp.CacheDir = s3.CacheDir
	}

	// --- Write flags ---
	if s3.AllowOverwrite {
		nodeServerOp = append(nodeServerOp, "--allow-overwrite")
		workerNodeOp.AllowOverwrite = "true"
	}
	if s3.IncrementalUpload {
		nodeServerOp = append(nodeServerOp, "--incremental-upload")
		workerNodeOp.IncrementalUpload = "true"
	}

	// --- Read-only ---
	if s3.ReadOnly {
		nodeServerOp = append(nodeServerOp, "--read-only")
		workerNodeOp.ReadOnly = "true"
	}

	// --- User passthrough (appended last — highest precedence) ---
	// Set on BOTH paths so the worker daemon receives passthrough flags too.
	nodeServerOp = append(nodeServerOp, s3.MountOptions...)
	workerNodeOp.Args = s3.MountOptions

	klog.Infof("MountpointS3 nodeServerOp: %v", nodeServerOp)
	klog.Infof("MountpointS3 workerNodeOp: %+v", workerNodeOp)

	return nodeServerOp, envVars, workerNodeOp
}

// removeS3MountConfigFile removes the config directory for the given volume target.
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
