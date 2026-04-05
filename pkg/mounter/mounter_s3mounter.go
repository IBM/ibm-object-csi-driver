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
//   - Directory validation: LogDirectory / CacheDir (must exist before mount)
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

	// LogDirectory must exist on the node before mount-s3 starts.
	// Validated in Mount() before calling formulateMountOptions.
	LogDirectory string

	// CacheDir must exist on the node before mount-s3 starts.
	// Validated in Mount() before calling formulateMountOptions.
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
// Only structured fields are represented here; passthrough flags are embedded
// in the Args slice at the call site.
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
//
// Source mapping and precedence (lowest → highest):
//
//  1. secretMap identity keys — accessKey, secretKey, bucketName, objectPath,
//     cosEndpoint, locationConstraint, uid, gid
//  2. StorageClass mountOptions (mountOptions []string arg) — parsed first
//  3. Secret mountOptions (secretMap["mountOptions"] multiline string) — parsed
//     second and therefore wins over StorageClass on any overlapping flag
//
// Special case: if read-only is present in either source, it takes priority and
// clears allow-overwrite and incremental-upload regardless of where they came from.
//
// For passthrough flags (unrecognised by the structured parser), the same
// precedence applies: secret passthroughs are appended after SC passthroughs,
// so mount-s3 sees the secret value last (last-value-wins semantics).
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
	// Step 1: StorageClass mountOptions (already split by kubelet).
	scRemainder := parseMountpointS3Options(mounter, mountOptions)

	// Step 2: Secret mountOptions (multiline string, parsed after SC so it wins).
	secretOpts := splitSecretMountOptions(secretMap["mountOptions"])
	secretRemainder := parseMountpointS3Options(mounter, secretOpts)

	// Step 3: SC passthroughs first, secret passthroughs last (last-value-wins).
	mounter.MountOptions = append(scRemainder, secretRemainder...)
	mounter.MounterUtils = mounterUtils

	klog.Infof("newMountpointS3Mounter args:\n\tbucketName: [%s]\n\tobjectPath: [%s]\n\tendPoint: [%s]\n\tlocationConstraint: [%s]\n\tauthType: [%s]",
		mounter.BucketName, mounter.ObjectPath, mounter.EndPoint, mounter.LocConstraint, mounter.AuthType)
	klog.Infof("newMountpointS3Mounter: SC mountOptions=%v secretMountOptions=%v passthrough=%v",
		mountOptions, secretOpts, mounter.MountOptions)

	return mounter
}

// splitSecretMountOptions splits the multiline mountOptions string from the
// secret into a []string suitable for parseMountpointS3Options.
//
// Rules:
//   - Each non-empty line is one flag.
//   - Lines beginning with '#' (after trimming) are comments — skipped.
//   - Leading/trailing whitespace on each line is trimmed.
//   - Blank lines are skipped.
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
//
// Only flags that require code-level handling are matched by name — everything
// else is forwarded unchanged to mount-s3 via the returned remainder slice.
//
// The remainder always has a "--" prefix added when missing, so mount-s3
// receives syntactically valid flags regardless of how the user wrote them.
// This handles both boolean flags ("allow-delete" → "--allow-delete") and
// key-value pairs ("max-threads=32" → "--max-threads=32") correctly because
// the original opt string is preserved and only the prefix is fixed.
//
// Rules:
//   - Flags may be provided with or without a leading "--" prefix.
//   - Boolean flags need no value.
//   - Value flags must be in "flag=value" or "--flag=value" form.
//   - "allow-other" is silently consumed — formulateMountOptions always adds it.
//   - "umask" is consumed with a warning — mount-s3 has no --umask flag.
//   - uid/gid override secretMap values (caller sets secretMap values first).
func parseMountpointS3Options(mounter *MountpointS3Mounter, opts []string) []string {
	var remaining []string

	for _, opt := range opts {
		// Normalise: strip optional leading "--" so switch cases are uniform.
		trimmed := strings.TrimPrefix(opt, "--")
		key, value, hasValue := strings.Cut(trimmed, "=")

		switch key {

		// ------------------------------------------------------------------
		// Always emitted — consume to avoid duplicates.
		// ------------------------------------------------------------------
		case "allow-other":
			// formulateMountOptions unconditionally adds --allow-other.

		// ------------------------------------------------------------------
		// Invalid for mount-s3 — warn and drop so mount-s3 doesn't fail.
		// ------------------------------------------------------------------
		case "umask":
			klog.Warningf("parseMountpointS3Options: 'umask' is not supported by mount-s3. " +
				"Use --dir-mode / --file-mode instead. Ignoring.")

		// ------------------------------------------------------------------
		// Identity — mountOptions values override secretMap values.
		// ------------------------------------------------------------------
		case "uid":
			if hasValue {
				mounter.UID = value
			}
		case "gid":
			if hasValue {
				mounter.GID = value
			}

		// ------------------------------------------------------------------
		// Log level — requires name translation in formulateMountOptions.
		// mount-s3 has no --log-level flag; valid values map to separate flags:
		//   "debug" → --debug, "debug-crt" → --debug-crt, "no-log" → --no-log
		// ------------------------------------------------------------------
		case "log-level":
			if hasValue {
				mounter.LogLevel = value
			}

		// ------------------------------------------------------------------
		// Directory flags — validated for existence before mount-s3 starts.
		// ------------------------------------------------------------------
		case "log-directory":
			if hasValue {
				mounter.LogDirectory = value
			}
		case "cache":
			if hasValue {
				mounter.CacheDir = value
			}

		// ------------------------------------------------------------------
		// Conflict trio — tracked for read-only priority resolution in Mount().
		// read-only clears allow-overwrite and incremental-upload if all three
		// are set, so SC defaults don't block users from requesting read-only.
		// ------------------------------------------------------------------
		case "read-only":
			mounter.ReadOnly = true
		case "allow-overwrite":
			mounter.AllowOverwrite = true
		case "incremental-upload":
			mounter.IncrementalUpload = true

		// ------------------------------------------------------------------
		// Everything else — pass through to mount-s3 unchanged.
		// Ensure "--" prefix so mount-s3 always receives a valid flag.
		// Works for both boolean flags and key-value pairs:
		//   "allow-delete"   → "--allow-delete"
		//   "max-threads=32" → "--max-threads=32"
		// ------------------------------------------------------------------
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
	// If read-only is set, it takes priority over all write-related flags.
	// AllowOverwrite and IncrementalUpload are cleared here, and allow-delete
	// is filtered from MountOptions, so the mount succeeds cleanly in read-only mode.
	// This means users can always request read-only via their secret even when
	// the StorageClass has write flags as defaults.
	if s3.ReadOnly {
		if s3.AllowOverwrite {
			klog.Infof("MountpointS3Mounter: read-only set — clearing allow-overwrite")
			s3.AllowOverwrite = false
		}
		if s3.IncrementalUpload {
			klog.Infof("MountpointS3Mounter: read-only set — clearing incremental-upload")
			s3.IncrementalUpload = false
		}

		// Filter out allow-delete from passthrough options
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

	// --- Directory validation ---
	// mount-s3 fails with a confusing error if these directories don't exist.
	// Validate here to surface a clear error message before spawning the process.
	if s3.LogDirectory != "" {
		if _, err := Stat(s3.LogDirectory); err != nil {
			return fmt.Errorf("MountpointS3Mounter: log-directory %q does not exist or is not accessible: %w",
				s3.LogDirectory, err)
		}
	}
	if s3.CacheDir != "" {
		if _, err := Stat(s3.CacheDir); err != nil {
			return fmt.Errorf("MountpointS3Mounter: cache directory %q does not exist or is not accessible: %w",
				s3.CacheDir, err)
		}
	}

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

	// Build bucket path with optional object path prefix.
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

	klog.Warning("MounterUtils does not implement FuseMountWithEnv, falling back to FuseMount. " +
		"Ensure AWS_SHARED_CREDENTIALS_FILE and AWS_CONFIG_FILE are set in the process environment.")
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
// mount-s3 locates these via AWS_SHARED_CREDENTIALS_FILE and AWS_CONFIG_FILE env vars.
//
// Directory structure:
//
//	<configPathWithVolID>/
//	  credentials   <- AWS credentials (access/secret key)
//	  config        <- AWS config (region, endpoint_url)
func createS3MountConfig(configPathWithVolID string, s3 *MountpointS3Mounter) error {
	if err := MakeDir(configPathWithVolID, 0755); err != nil { // #nosec G301
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

// formulateMountOptions builds the CLI args, env vars, and worker payload for mount-s3.
//
// Only structured fields are emitted here. All other user-supplied flags arrive
// pre-formatted in s3.MountOptions and are appended at the end.
//
// By the time this is called, Mount() has already:
//   - Cleared AllowOverwrite / IncrementalUpload if ReadOnly is set
//   - Validated LogDirectory and CacheDir existence
//
// Flag ordering:
//  1. Positional args   : bucket, mountpoint
//  2. Always-on         : --allow-other
//  3. SecretMap-sourced : --endpoint-url, --region
//  4. Identity          : --uid, --gid
//  5. Logging           : --debug/--debug-crt/--no-log, --log-directory
//  6. Cache             : --cache
//  7. Write flags       : --allow-overwrite, --incremental-upload
//  8. Read-only         : --read-only
//  9. User passthrough  : s3.MountOptions (appended last — highest precedence)
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
	// mount-s3 has no --log-level flag; each level maps to its own flag.
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

	// --- Log directory (existence validated in Mount()) ---
	if s3.LogDirectory != "" {
		nodeServerOp = append(nodeServerOp, "--log-directory="+s3.LogDirectory)
		workerNodeOp.LogDirectory = s3.LogDirectory
	}

	// --- Cache directory (existence validated in Mount()) ---
	if s3.CacheDir != "" {
		nodeServerOp = append(nodeServerOp, "--cache="+s3.CacheDir)
		workerNodeOp.CacheDir = s3.CacheDir
	}

	// --- Write flags ---
	// These are only emitted if ReadOnly is false.
	// Mount() clears these when ReadOnly is set, so no conflict is possible here.
	if s3.AllowOverwrite {
		nodeServerOp = append(nodeServerOp, "--allow-overwrite")
		workerNodeOp.AllowOverwrite = "true"
	}
	if s3.IncrementalUpload {
		nodeServerOp = append(nodeServerOp, "--incremental-upload")
		workerNodeOp.IncrementalUpload = "true"
	}

	// --- Read-only ---
	// Safe to emit here — Mount() already cleared AllowOverwrite and
	// IncrementalUpload above, so no conflicting flags will be present.
	if s3.ReadOnly {
		nodeServerOp = append(nodeServerOp, "--read-only")
		workerNodeOp.ReadOnly = "true"
	}

	// --- User passthrough (appended last — highest precedence) ---
	// All entries already have "--" prefix (enforced by parseMountpointS3Options).
	nodeServerOp = append(nodeServerOp, s3.MountOptions...)

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
