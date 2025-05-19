package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger     *zap.Logger
	socketPath = "/var/lib/coscsi.sock"

	s3fs   = "s3fs"
	rclone = "rclone"
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	logger = setUpLogger()
	defer logger.Sync()
}

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
	), zap.AddCaller()).With(zap.String("ServiceName", "cos-csi-mounter-service"))
	atom.SetLevel(zap.InfoLevel)
	return logger
}

func main() {
	// Always create fresh socket file
	err := os.Remove(socketPath)
	if err != nil {
		// Handle it properly: log it, retry, return, etc.
		logger.Warn("Failed to remove Socket File")
	}

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
		err := os.Remove(socketPath)
		if err != nil {
			// Handle it properly: log it, retry, return, etc.
			logger.Warn("Failed to remove Socket File")
		}
		os.Exit(0)
	}()

	logger.Info("Starting cos-csi-mounter service...")

	// Create gin router
	router := gin.Default()

	router.POST("/api/cos/mount", handleCosMount())
	router.POST("/api/cos/unmount", handleCosUnmount())

	// Serve HTTP requests over Unix socket
	// err = http.Serve(listener, router)
	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 3 * time.Second,
	}
	err = server.Serve(listener)
	if err != nil {
		logger.Fatal("Error while serving HTTP requests:", zap.Error(err))
	}
}

func handleCosMount() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request MountRequest

		if err := c.BindJSON(&request); err != nil {
			logger.Error("invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		logger.Info("New mount request with values:", zap.String("Bucket", request.Bucket), zap.String("Path", request.Path), zap.String("Mounter", request.Mounter), zap.Any("Args", request.Args))

		if request.Mounter != s3fs && request.Mounter != rclone {
			logger.Error("invalid mounter", zap.Any("mounter", request.Mounter))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mounter"})
			return
		}

		// validate mounter args
		args, err := request.ParseMounterArgs()
		if err != nil {
			logger.Error("failed to parse mounter args", zap.Any("mounter", request.Mounter), zap.Error(err))

			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid args for mounter: %v", err)})
			return
		}

		utils := mounterUtils.MounterOptsUtils{}
		err = utils.FuseMount(request.Path, request.Mounter, args)
		if err != nil {
			logger.Error("mount failed: ", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("mount failed: %v", err)})
			return
		}

		logger.Info("bucket mount is successful", zap.Any("bucket", request.Bucket), zap.Any("path", request.Path))
		c.JSON(http.StatusOK, "Success!!")
	}
}

func handleCosUnmount() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Path string `json:"path"`
		}

		if err := c.BindJSON(&request); err != nil {
			logger.Error("invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		logger.Info("New unmount request with values: ", zap.String("Path:", request.Path))

		utils := mounterUtils.MounterOptsUtils{}
		err := utils.FuseUnmount(request.Path)
		if err != nil {
			logger.Error("unmount failed: ", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("unmount failed :%v", err)})
			return
		}

		logger.Info("bucket unmount is successful", zap.Any("path", request.Path))
		c.JSON(http.StatusOK, "Success!!")
	}
}
