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
type MountpointS3Mounter struct {
	BucketName     string
	ObjectPath     string
	EndPoint       string
	LocConstraint  string
	AuthType       string
	AccessKey      string
	SecretKey      string
	UID            string
	GID            string
	LogLevel       string // valid values: "debug", "debug-crt", "no-log"; mount-s3 has no --log-level flag
	LogDirectory   string // --log-directory <dir>, write logs to directory instead of syslog
	ReadOnly       bool
	AllowDelete    bool // --allow-delete, enable delete operations
	AllowOverwrite bool // --allow-overwrite, enable overwrite operations
	MountOptions   []string
	MounterUtils   utils.MounterUtils

	MaxThreads          string // --max-threads <N>, default: 16
	ReadPartSize        string // --read-part-size <bytes>, default: 8388608 (8 MiB)
	WritePartSize       string // --write-part-size <bytes>, default: 8388608 (8 MiB)
	MaxThroughputGbps   string // --maximum-throughput-gbps <N>, default: auto-detect on EC2, 10 Gbps elsewhere
	UploadChecksums     string // --upload-checksums <crc32c|off>, default: crc32c
	CacheDir            string // --cache <dir>, enables local disk read cache, default: disabled
	MaxCacheSize        string // --max-cache-size <MiB>, only effective if CacheDir is set, default: preserve 5% of available space
	MetadataTTL         string // --metadata-ttl <seconds|indefinite|minimal>, default: minimal (or 60s if --cache is set)
	NegativeMetadataTTL string // --negative-metadata-ttl <seconds|indefinite|minimal>, default: same as MetadataTTL
	LogMetrics          bool   // --log-metrics, enables summarized performance metrics in logs, default: false
}

// s3MounterArgs holds the args passed to the worker for mount-s3.
type s3MounterArgs struct {
	ReadOnly   string `json:"read-only,omitempty"`
	AllowOther string `json:"allow-other,omitempty"`
	UID        string `json:"uid,omitempty"`
	GID        string `json:"gid,omitempty"`
	// Removed: UMask        — mount-s3 has no --umask flag; use --dir-mode/--file-mode via MountOptions
	// Removed: AwsConfigDir — mount-s3 has no --aws-config-dir flag
	LogLevel       string `json:"log-level,omitempty"` // valid: "debug", "debug-crt", "no-log"
	EndpointURL    string `json:"endpoint-url,omitempty"`
	AllowDelete    string `json:"allow-delete,omitempty"`    // --allow-delete flag
	AllowOverwrite string `json:"allow-overwrite,omitempty"` // --allow-overwrite flag
	LogDirectory   string `json:"log-directory,omitempty"`   // --log-directory flag
	// Region is passed explicitly as --region CLI flag in addition to being written
	// in the AWS config file, to ensure it is always set even if AWS_CONFIG_FILE
	// env var is not propagated correctly to the mount-s3 subprocess.
	Region             string `json:"region,omitempty"`
	AwsCredentialsFile string `json:"aws-credentials-file,omitempty"`
	AwsConfigFile      string `json:"aws-config-file,omitempty"`

	// Performance tuning fields — all optional, mirrors MountpointS3Mounter fields.
	MaxThreads          string `json:"max-threads,omitempty"`
	ReadPartSize        string `json:"read-part-size,omitempty"`
	WritePartSize       string `json:"write-part-size,omitempty"`
	MaxThroughputGbps   string `json:"maximum-throughput-gbps,omitempty"`
	UploadChecksums     string `json:"upload-checksums,omitempty"`
	CacheDir            string `json:"cache,omitempty"`
	MaxCacheSize        string `json:"max-cache-size,omitempty"`
	MetadataTTL         string `json:"metadata-ttl,omitempty"`
	NegativeMetadataTTL string `json:"negative-metadata-ttl,omitempty"`
	LogMetrics          bool   `json:"log-metrics,omitempty"`
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
// For passthrough flags (unrecognised by the parser), the same precedence
// applies: secret passthroughs are appended after SC passthroughs, so
// mount-s3 sees the secret value last (last-value-wins semantics).
func NewMountpointS3Mounter(secretMap map[string]string, mountOptions []string, mounterUtils utils.MounterUtils) Mounter {
	klog.Info("-newMountpointS3Mounter-")

	mounter := &MountpointS3Mounter{}

	// --- Credentials (unified secret `data` fields, base64-decoded by k8s) ---
	if val, ok := secretMap["accessKey"]; ok {
		mounter.AccessKey = val
	}
	if val, ok := secretMap["secretKey"]; ok {
		mounter.SecretKey = val
	}

	// --- Bucket / endpoint identity (`stringData` fields) ---
	if val, ok := secretMap["bucketName"]; ok {
		mounter.BucketName = val
	}
	if val, ok := secretMap["objectPath"]; ok {
		mounter.ObjectPath = val
	}
	// cosEndpoint and locationConstraint are IBM COS-specific.
	// They are not part of the upstream unified secret schema but remain
	// supported as secretMap keys for IBM COS compatibility.
	if val, ok := secretMap["cosEndpoint"]; ok {
		mounter.EndPoint = val
	}
	if val, ok := secretMap["locationConstraint"]; ok {
		mounter.LocConstraint = val
	}

	// --- UID / GID from secretMap (lowest precedence for these fields) ---
	// If gid is set but uid is not, mirror gid → uid (matches rclone behavior).
	// Both can be overridden by mountOptions below.
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
	//
	// Step 1: Parse StorageClass mountOptions (the []string arg).
	//         These come in already split by the kubelet.
	scRemainder := parseMountpointS3Options(mounter, mountOptions)

	// Step 2: Parse the secret's mountOptions multiline string.
	//         splitSecretMountOptions handles blank lines and # comments.
	//         Secret options are parsed AFTER SC options so they win on overlap.
	secretOpts := splitSecretMountOptions(secretMap["mountOptions"])
	secretRemainder := parseMountpointS3Options(mounter, secretOpts)

	// Step 3: Build the final passthrough slice.
	//         SC passthroughs come first; secret passthroughs come last so
	//         mount-s3 sees the secret value last (last-value-wins).
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
//   - Each non-empty line is one flag (same as StorageClass mountOptions).
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

// parseMountpointS3Options parses mount-s3 options from the provided slice and
// populates recognised flags into the mounter struct.
//
// Rules:
//   - Flags may be provided with or without a leading "--" prefix (both accepted).
//   - Boolean flags (--read-only, --allow-delete, etc.) need no value.
//   - Value flags must be in "flag=value" or "--flag=value" form.
//   - "allow-other" is silently consumed — formulateMountOptions always adds it.
//   - "umask" is consumed with a warning — mount-s3 has no --umask flag.
//   - Unrecognised flags are returned unchanged for pass-through to mount-s3.
//   - mountOptions uid/gid override secretMap uid/gid (caller sets secretMap
//     values first; this function runs second).
func parseMountpointS3Options(mounter *MountpointS3Mounter, opts []string) []string {
	var remaining []string

	for _, opt := range opts {
		// Normalise: strip optional leading "--" so comparisons are uniform.
		trimmed := strings.TrimPrefix(opt, "--")

		key, value, hasValue := strings.Cut(trimmed, "=")

		switch key {

		// ------------------------------------------------------------------
		// Always emitted by formulateMountOptions — consume to avoid duplicates.
		// ------------------------------------------------------------------
		case "allow-other":
			// formulateMountOptions unconditionally adds --allow-other; drop here.

		// ------------------------------------------------------------------
		// Unsupported by mount-s3 — warn and drop so mount-s3 doesn't fail.
		// ------------------------------------------------------------------
		case "umask":
			klog.Warningf("parseMountpointS3Options: 'umask' is not supported by mount-s3. " +
				"Use --dir-mode / --file-mode via mountOptions in the StorageClass instead. Ignoring.")

		// ------------------------------------------------------------------
		// Boolean mount flags — presence means enabled, no value needed.
		// ------------------------------------------------------------------
		case "read-only":
			mounter.ReadOnly = true
		case "allow-delete":
			mounter.AllowDelete = true
		case "allow-overwrite":
			mounter.AllowOverwrite = true
		case "log-metrics":
			mounter.LogMetrics = true

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
		// Logging
		// ------------------------------------------------------------------
		case "log-level":
			// mount-s3 has no --log-level flag.
			// formulateMountOptions maps: "debug" → --debug,
			// "debug-crt" → --debug-crt, "no-log" → --no-log.
			if hasValue {
				mounter.LogLevel = value
			}
		case "log-directory":
			if hasValue {
				mounter.LogDirectory = value
			}

		// ------------------------------------------------------------------
		// Performance tuning
		// ------------------------------------------------------------------
		case "max-threads":
			if hasValue {
				mounter.MaxThreads = value
			}
		case "read-part-size":
			if hasValue {
				mounter.ReadPartSize = value
			}
		case "write-part-size":
			if hasValue {
				mounter.WritePartSize = value
			}
		case "maximum-throughput-gbps":
			if hasValue {
				mounter.MaxThroughputGbps = value
			}
		case "upload-checksums":
			if hasValue {
				mounter.UploadChecksums = value
			}

		// ------------------------------------------------------------------
		// Caching
		// ------------------------------------------------------------------
		case "cache":
			if hasValue {
				mounter.CacheDir = value
			}
		case "max-cache-size":
			if hasValue {
				mounter.MaxCacheSize = value
			}
		case "metadata-ttl":
			if hasValue {
				mounter.MetadataTTL = value
			}
		case "negative-metadata-ttl":
			if hasValue {
				mounter.NegativeMetadataTTL = value
			}

		// ------------------------------------------------------------------
		// Unrecognised — pass through to mount-s3 unchanged.
		// ------------------------------------------------------------------
		default:
			remaining = append(remaining, opt)
		}
	}

	return remaining
}

// Mount mounts the S3 bucket using mountpoint-s3.
func (s3 *MountpointS3Mounter) Mount(source string, target string) error {
	klog.Info("-MountpointS3Mounter Mount-")
	klog.Infof("Mount args:\n\tsource: <%s>\n\ttarget: <%s>", source, target)

	// Determine config path based on mode.
	var configPath string
	if mountWorker {
		configPath = constants.MounterConfigPathOnHost
	} else {
		configPath = constants.MounterConfigPathOnPodS3Mount
	}

	// Create unique config dir per volume.
	configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(target))))

	// Write AWS credentials and config files.
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

	// Use FuseMountWithEnv if available on the concrete type (race-safe, per-subprocess env).
	// Falls back to FuseMount if running under a mock or different implementation — the
	// fallback will still work but credentials must be available via system env in that case.
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
	// Create the config directory.
	if err := MakeDir(configPathWithVolID, 0755); err != nil { // #nosec G301 -- directory is created under a controlled path using SHA256 hash, not user input
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
	f, err := CreateFile(filePath) // #nosec G304 -- filePath is constructed internally from a SHA256 hash and constant config dir, not from user input
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			klog.Errorf("writeConfigFile: cannot close file %s: %v", filePath, cerr)
		}
	}()

	if err := Chmod(filePath, 0600); err != nil { // #nosec G302 -- restrictive permissions (0600) are intentionally set for credential files
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

// formulateMountOptions builds CLI args, env vars, and the worker payload for mount-s3.
//
// Removed flags (do not exist in mount-s3):
//   - --aws-config-dir  → replaced by AWS_SHARED_CREDENTIALS_FILE / AWS_CONFIG_FILE env vars
//   - --log-level       → mapped to --debug / --debug-crt / --no-log
//   - --umask           → use --dir-mode / --file-mode via MountOptions in StorageClass
func (s3 *MountpointS3Mounter) formulateMountOptions(bucket, target, configPathWithVolID string) (nodeServerOp []string, envVars []string, workerNodeOp s3MounterArgs) {
	// mount-s3 syntax: mount-s3 <bucket> <mountpoint> [flags]
	nodeServerOp = []string{
		bucket,
		target,
		"--allow-other",
	}

	// AWS credentials via env vars — mount-s3 and the underlying AWS SDK reads these
	// to locate the credentials and config files written by createS3MountConfig.
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

	// Endpoint (IBM COS endpoint).
	if s3.EndPoint != "" {
		nodeServerOp = append(nodeServerOp, "--endpoint-url="+s3.EndPoint)
		workerNodeOp.EndpointURL = s3.EndPoint
	}

	// Region — passed as CLI flag AND written to config file for redundancy.
	if s3.LocConstraint != "" {
		nodeServerOp = append(nodeServerOp, "--region="+s3.LocConstraint)
		workerNodeOp.Region = s3.LocConstraint
	}

	// UID.
	if s3.UID != "" {
		nodeServerOp = append(nodeServerOp, "--uid="+s3.UID)
		workerNodeOp.UID = s3.UID
	}

	// GID.
	if s3.GID != "" {
		nodeServerOp = append(nodeServerOp, "--gid="+s3.GID)
		workerNodeOp.GID = s3.GID
	}

	// Log level: mount-s3 has no --log-level flag.
	// Valid mappings: "debug" → --debug, "debug-crt" → --debug-crt, "no-log" → --no-log.
	// Anything else (e.g. "warn", "info") is ignored — mount-s3 default logging applies.
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
			klog.Warningf("MountpointS3Mounter: unsupported log level '%s' for mount-s3. "+
				"Supported values: 'debug', 'debug-crt', 'no-log'. Ignoring.", s3.LogLevel)
		}
	}

	// Read-only.
	if s3.ReadOnly {
		nodeServerOp = append(nodeServerOp, "--read-only")
		workerNodeOp.ReadOnly = "true"
	}

	// Allow delete.
	if s3.AllowDelete {
		nodeServerOp = append(nodeServerOp, "--allow-delete")
		workerNodeOp.AllowDelete = "true"
	}

	// Allow overwrite.
	if s3.AllowOverwrite {
		nodeServerOp = append(nodeServerOp, "--allow-overwrite")
		workerNodeOp.AllowOverwrite = "true"
	}

	// Log directory.
	if s3.LogDirectory != "" {
		nodeServerOp = append(nodeServerOp, "--log-directory="+s3.LogDirectory)
		workerNodeOp.LogDirectory = s3.LogDirectory
	}

	// --- Performance tuning options ---

	// --read-part-size (default: 8 MiB).
	if s3.ReadPartSize != "" {
		nodeServerOp = append(nodeServerOp, "--read-part-size="+s3.ReadPartSize)
		workerNodeOp.ReadPartSize = s3.ReadPartSize
	}

	// --write-part-size (default: 8 MiB).
	if s3.WritePartSize != "" {
		nodeServerOp = append(nodeServerOp, "--write-part-size="+s3.WritePartSize)
		workerNodeOp.WritePartSize = s3.WritePartSize
	}

	// --max-threads (default: 16).
	if s3.MaxThreads != "" {
		nodeServerOp = append(nodeServerOp, "--max-threads="+s3.MaxThreads)
		workerNodeOp.MaxThreads = s3.MaxThreads
	}

	// --maximum-throughput-gbps (default: auto-detect on EC2, 10 Gbps elsewhere).
	// Not on EC2 so won't auto-detect correctly — set explicitly if you know your node bandwidth.
	if s3.MaxThroughputGbps != "" {
		nodeServerOp = append(nodeServerOp, "--maximum-throughput-gbps="+s3.MaxThroughputGbps)
		workerNodeOp.MaxThroughputGbps = s3.MaxThroughputGbps
	}

	// --upload-checksums (default: crc32c).
	// Set to "off" to reduce CPU overhead on write-heavy workloads at the cost of integrity checks.
	if s3.UploadChecksums != "" {
		nodeServerOp = append(nodeServerOp, "--upload-checksums="+s3.UploadChecksums)
		workerNodeOp.UploadChecksums = s3.UploadChecksums
	}

	// --cache + --max-cache-size (default: disabled).
	// Enables local disk read cache — improves repeated read performance.
	// When cache is set, metadata-ttl defaults to 60s automatically.
	if s3.CacheDir != "" {
		nodeServerOp = append(nodeServerOp, "--cache="+s3.CacheDir)
		workerNodeOp.CacheDir = s3.CacheDir
		if s3.MaxCacheSize != "" {
			nodeServerOp = append(nodeServerOp, "--max-cache-size="+s3.MaxCacheSize)
			workerNodeOp.MaxCacheSize = s3.MaxCacheSize
		}
	}

	// --metadata-ttl (default: minimal, or 60s if --cache is set).
	// Set higher to reduce ListObjects API calls on stable buckets.
	if s3.MetadataTTL != "" {
		nodeServerOp = append(nodeServerOp, "--metadata-ttl="+s3.MetadataTTL)
		workerNodeOp.MetadataTTL = s3.MetadataTTL
	}

	// --negative-metadata-ttl (default: same as metadata-ttl).
	if s3.NegativeMetadataTTL != "" {
		nodeServerOp = append(nodeServerOp, "--negative-metadata-ttl="+s3.NegativeMetadataTTL)
		workerNodeOp.NegativeMetadataTTL = s3.NegativeMetadataTTL
	}

	// --log-metrics (default: false).
	// Logs summarized throughput metrics — zero performance cost, useful for monitoring.
	if s3.LogMetrics {
		nodeServerOp = append(nodeServerOp, "--log-metrics")
		workerNodeOp.LogMetrics = true
	}

	// StorageClass mountOptions are appended LAST — always take final precedence over everything above.
	nodeServerOp = append(nodeServerOp, s3.MountOptions...)

	klog.Infof("MountpointS3 nodeServerOp: %v", nodeServerOp)
	klog.Infof("MountpointS3 workerNodeOp: %+v", workerNodeOp)

	return nodeServerOp, envVars, workerNodeOp
}

// removeS3MountConfigFile removes the config directory for the volume (same pattern as rclone).
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
