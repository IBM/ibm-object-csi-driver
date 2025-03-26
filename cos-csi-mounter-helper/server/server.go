package main

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	logger = setUpLogger()
	defer logger.Sync()
}

var (
	logger     *zap.Logger
	socketDir  = "/var/lib/"
	socketPath = socketDir + "ibmshare.sock"

	s3fs   = "s3fs"
	rclone = "rclone"
	// metaRoot       = "/var/lib/ibmc-s3fs"
	metaRoot       = "/var/lib/s3fs"
	passFile       = ".passwd-s3fs" // #nosec G101: not password
	metaRootRclone = "/var/lib/ibmc-rclone"
	configPath     = "/root/.config/rclone"
	configFileName = "rclone.conf"
	remote         = "ibmcos"
	s3Type         = "s3"
	cosProvider    = "IBMCOS"
	envAuth        = "true"
)

func setUpLogger() *zap.Logger {
	// Prepare a new logger
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller()).With(zap.String("ServiceName", "mount-helper-conatiner-service"))
	atom.SetLevel(zap.InfoLevel)
	return logger
}

func main() {
	// Always create fresh socket file
	os.Remove(socketPath)

	// Create a listener
	logger.Info("Creating unix socket listener...")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		logger.Fatal("Failed to create unix socket listener:", zap.Error(err))
	}
	// Close the listener at the end
	defer listener.Close()

	// Handle SIGINT and SIGTERM signals to gracefully shut down the server
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		os.Remove(socketPath)
		os.Exit(0)
	}()

	logger.Info("Starting cos-mount-helper service...")

	// Create gin router
	router := gin.Default()

	router.POST("/api/cos/mount", handleCosMount())
	router.POST("/api/cos/unmount", handleCosUnmount())

	// Serve HTTP requests over Unix socket
	err = http.Serve(listener, router)
	if err != nil {
		logger.Fatal("Error while serving HTTP requests:", zap.Error(err))
	}
}

func handleCosMount() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Path      string   `json:"path"`
			Mounter   string   `json:"mounter"`
			Args      []string `json:"args"`
			APIKey    string   `json:"apiKey"`
			AccessKey string   `json:"accessKey"`
			SecretKey string   `json:"secretKey"`
		}

		logger.Info("New mount request with values: ", zap.Any("Request:", request))

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New mount request with values: ", zap.String("Path:", request.Path), zap.String("Mounter:", request.Mounter), zap.Any("Args:", request.Args))

		var pathExist bool
		var err error
		var mounterType string

		if request.Mounter == s3fs+"-mounter" {
			logger.Info("MOUNTER S3FS...")
			metaPath := path.Join(metaRoot, fmt.Sprintf("%x", sha256.Sum256([]byte(request.Path))))
			logger.Info("metaRoot", zap.String("", metaRoot))
			pathExist, err = checkPath(metaPath)
			logger.Info("pathExist", zap.Bool("", pathExist))
			if err != nil {
				logger.Error("S3FSMounter Mount: Cannot stat directory", zap.String("metaPath", metaPath), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
				return
			}

			logger.Info("metaPath", zap.String("", metaPath))

			if !pathExist {
				err = mkdirAll(metaPath, 0755) // #nosec G301: used for s3fs
				if err != nil {
					logger.Error("S3FSMounter Mount: Cannot create directory", zap.String("metaPath", metaPath), zap.Error(err))
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
					return
				} else {
					logger.Info("S3FSMounter Mount: MetaPath created Successfully...")
				}
			}

			logger.Info("path created")

			accessKeys := ""
			if request.APIKey != "" {
				accessKeys = fmt.Sprintf(":%s", request.APIKey)
			} else {
				accessKeys = fmt.Sprintf("%s:%s", request.AccessKey, request.SecretKey)
			}

			passwdFile := path.Join(metaPath, passFile)
			if err = writePassWrap(passwdFile, accessKeys); err != nil {
				logger.Error("S3FSMounter Mount: Cannot create file", zap.String("metaPath", metaPath), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
				return
			}

			mounterType = s3fs

			logger.Info("pass file", zap.String("passwdFile", passwdFile))
		} else if request.Mounter == rclone+"-mounter" {
			logger.Info("MOUNTER RCLONE...")
			metaPath := path.Join(metaRootRclone, fmt.Sprintf("%x", sha256.Sum256([]byte(request.Path))))
			if pathExist, err = checkPath(metaPath); err != nil {
				logger.Error("RcloneMounter Mount: Cannot stat directory", zap.String("metaPath", metaPath), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
				return
			}

			if !pathExist {
				if err = mkdirAll(metaPath, 0755); // #nosec G301: used for rclone
				err != nil {
					logger.Error("RcloneMounter Mount: Cannot create directory", zap.String("metaPath", metaPath), zap.Error(err))
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
					return
				}
			}

			accessKeys := fmt.Sprintf("%s:%s", request.AccessKey, request.SecretKey)

			configPathWithVolID := path.Join(configPath, fmt.Sprintf("%x", sha256.Sum256([]byte(request.Path))))
			if err = createConfigWrap(configPathWithVolID, accessKeys); err != nil {
				logger.Error("RcloneMounter Mount: Cannot create rclone config file %v", zap.String("metaPath", metaPath), zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
				return
			}

			mounterType = rclone

			logger.Info("configPathWithVolID", zap.String("configPathWithVolID", configPathWithVolID))
		} else {
			logger.Error("Invalid Request!!!!")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		utils := mounterUtils.MounterOptsUtils{}
		err = utils.FuseMount(request.Path, mounterType, request.Args)
		if err != nil {
			logger.Error("Mount Failed: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mount Failed"})
			return
		}

		c.JSON(http.StatusOK, "Success!!")
	}
}

func handleCosUnmount() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Path string `json:"path"`
		}

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New unmount request with values: ", zap.String("Path:", request.Path))

		utils := mounterUtils.MounterOptsUtils{}
		err := utils.FuseUnmount(request.Path)
		if err != nil {
			logger.Error("Mount Failed: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mount Failed"})
			return
		}

		c.JSON(http.StatusOK, "Success!!")
	}
}

func checkPath(path string) (bool, error) {
	logger.Info("In checkPath")
	if path == "" {
		logger.Info("In checkPath, path" + path)
		return false, errors.New("undefined path")
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else if isCorruptedMnt(err) {
		return true, err
	}
	return false, err
}

func isCorruptedMnt(err error) bool {
	if err == nil {
		return false
	}
	var underlyingError error
	switch pe := err.(type) {
	case *os.PathError:
		underlyingError = pe.Err
	case *os.LinkError:
		underlyingError = pe.Err
	case *os.SyscallError:
		underlyingError = pe.Err
	}
	return underlyingError == syscall.ENOTCONN || underlyingError == syscall.ESTALE
}

var mkdirAllFunc = os.MkdirAll

// Function that wraps os.MkdirAll
var mkdirAll = func(path string, perm os.FileMode) error {
	return mkdirAllFunc(path, perm)
}

var writePassFunc = writePass

// Function that wraps writePass
var writePassWrap = func(pwFileName string, pwFileContent string) error {
	return writePassFunc(pwFileName, pwFileContent)
}

func writePass(pwFileName string, pwFileContent string) error {
	pwFile, err := os.OpenFile(pwFileName, os.O_RDWR|os.O_CREATE, 0600) // #nosec G304: Value is dynamic
	if err != nil {
		return err
	}
	_, err = pwFile.WriteString(pwFileContent)
	if err != nil {
		return err
	}
	err = pwFile.Close() // #nosec G304: Value is dynamic
	if err != nil {
		return err
	}
	return nil
}

var createConfigFunc = createConfig

// Function that wraps writePass
var createConfigWrap = func(configPathWithVolID, accessKeys string) error {
	return createConfigFunc(configPathWithVolID, accessKeys)
}

func createConfig(configPathWithVolID, accessKeys string) error {
	var accessKey string
	var secretKey string
	keys := strings.Split(accessKeys, ":")
	if len(keys) == 2 {
		accessKey = keys[0]
		secretKey = keys[1]
	}
	configParams := []string{
		"[" + remote + "]",
		"type = " + s3Type,
		"endpoint = " + "https://s3.direct.au-syd.cloud-object-storage.appdomain.cloud", // FIX
		"provider = " + cosProvider,
		"env_auth = " + envAuth,
		"location_constraint = " + "au-syd-standard", // FIX
		"access_key_id = " + accessKey,
		"secret_access_key = " + secretKey,
	}

	// configParams = append(configParams, rclone.MountOptions...)  // FIX
	configParams = append(configParams, "acl=private", "bucket_acl=private", "upload_cutoff=256Mi", "chunk_size=64Mi",
		"max_upload_parts=64", "upload_concurrency=20", "copy_cutoff=1Gi", "memory_pool_flush_time=30s", "disable_checksum=true")

	if err := os.MkdirAll(configPathWithVolID, 0755); // #nosec G301: used for rclone
	err != nil {
		logger.Error("RcloneMounter Mount: Cannot create directory", zap.String("configPathWithVolID", configPathWithVolID), zap.Error(err))
		return err
	}

	configFile := path.Join(configPathWithVolID, configFileName)
	file, err := os.Create(configFile) // #nosec G304 used for rclone
	if err != nil {
		logger.Error("RcloneMounter Mount: Cannot create file", zap.String("configFileName", configFileName), zap.Error(err))
		return err
	}
	defer func() {
		if err = file.Close(); err != nil {
			logger.Error("RcloneMounter Mount: Cannot close file", zap.String("configFileName", configFileName), zap.Error(err))
		}
	}()

	err = os.Chmod(configFile, 0644) // #nosec G302: used for rclone
	if err != nil {
		logger.Error("RcloneMounter Mount: Cannot change permissions on file", zap.String("configFileName", configFileName), zap.Error(err))
		return err
	}

	logger.Info("-Rclone writing to config-")
	datawriter := bufio.NewWriter(file)
	for _, line := range configParams {
		_, err = datawriter.WriteString(line + "\n")
		if err != nil {
			logger.Error("RcloneMounter Mount: Could not write to config file: %v", zap.Error(err))
			return err
		}
	}
	err = datawriter.Flush()
	if err != nil {
		return err
	}
	logger.Info("-Rclone created rclone config file-")
	return nil
}
