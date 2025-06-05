package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger             *zap.Logger
	MakeDir            = os.MkdirAll
	RemoveFile         = os.Remove
	UnixSocketListener = func(network, address string) (net.Listener, error) {
		return net.Listen(network, address)
	}
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	logger = setUpLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to sync logger: %v\n", err)
		}
	}()
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
	), zap.AddCaller()).With(zap.String("ServiceName", "cos-csi-mounter"))
	atom.SetLevel(zap.InfoLevel)
	return logger
}

func setupSocket() (net.Listener, error) {
	socketPath := filepath.Join(constants.SocketDir, constants.SocketFile)

	// Ensure the socket directory exists
	if err := MakeDir(constants.SocketDir, 0750); err != nil {
		logger.Error("Failed to create socket directory", zap.String("dir", constants.SocketDir), zap.Error(err))
		return nil, err
	}

	// Check for socket file
	if _, err := os.Stat(socketPath); err == nil {
		if err := RemoveFile(socketPath); err != nil {
			logger.Warn("Failed to remove existing socket file", zap.String("path", socketPath), zap.Error(err))
		}
	} else if !os.IsNotExist(err) {
		logger.Warn("Could not stat socket file", zap.String("path", socketPath), zap.Error(err))
	}

	logger.Info("Creating unix socket listener...", zap.String("path", socketPath))
	listener, err := UnixSocketListener("unix", socketPath)
	if err != nil {
		logger.Error("Failed to create unix socket listener", zap.String("path", socketPath), zap.Error(err))
		return nil, err
	}
	return listener, nil
}

func handleSignals() {
	// Handle SIGINT and SIGTERM signals to gracefully shut down the server
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signals
		socketPath := filepath.Join(constants.SocketDir, constants.SocketFile)
		if err := os.Remove(socketPath); err != nil {
			// Handle it properly: log it, retry, return, etc.
			logger.Warn("Failed to remove socket on exit", zap.String("path", socketPath), zap.Error(err))
		}
		os.Exit(0)
	}()
}

func newRouter() *gin.Engine {
	utils := &mounterUtils.MounterOptsUtils{}
	parser := &DefaultMounterArgsParser{}

	// Create gin router
	router := gin.Default()
	router.POST("/api/cos/mount", handleCosMount(utils, parser))
	router.POST("/api/cos/unmount", handleCosUnmount(utils))
	return router
}

func startService(setupSocketFunc func() (net.Listener, error), router http.Handler, handleSignalsFunc func()) error {
	listener, err := setupSocketFunc()
	if err != nil {
		logger.Error("Failed to create socket", zap.Error(err))
		return err
	}
	// Close the listener at the end
	defer func() {
		if err := listener.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close listener: %v\n", err)
		}
	}()

	handleSignalsFunc()

	logger.Info("Starting cos-csi-mounter service...")

	// Serve HTTP requests over Unix socket
	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 3 * time.Second,
	}
	if err := server.Serve(listener); err != nil {
		logger.Error("Error while serving HTTP requests:", zap.Error(err))
		return err
	}
	return nil
}

func main() {
	err := startService(setupSocket, newRouter(), handleSignals)
	if err != nil {
		logger.Fatal("cos-csi-mounter exited with error", zap.Error(err))
	}
}

func handleCosMount(mounter mounterUtils.MounterUtils, parser MounterArgsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request MountRequest

		if err := c.BindJSON(&request); err != nil {
			logger.Error("invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		logger.Info("New mount request with values:", zap.String("Bucket", request.Bucket), zap.String("Path", request.Path), zap.String("Mounter", request.Mounter), zap.Any("Args", request.Args))

		if request.Mounter != constants.S3FS && request.Mounter != constants.RClone {
			logger.Error("invalid mounter", zap.Any("mounter", request.Mounter))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mounter"})
			return
		}

		if request.Bucket == "" {
			logger.Error("missing bucket in request")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing bucket"})
			return
		}

		// validate mounter args
		args, err := parser.Parse(request)
		if err != nil {
			logger.Error("failed to parse mounter args", zap.Any("mounter", request.Mounter), zap.Error(err))

			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid args for mounter: %v", err)})
			return
		}

		err = mounter.FuseMount(request.Path, request.Mounter, args)
		if err != nil {
			logger.Error("mount failed: ", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("mount failed: %v", err)})
			return
		}

		logger.Info("bucket mount is successful", zap.Any("bucket", request.Bucket), zap.Any("path", request.Path))
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func handleCosUnmount(mounter mounterUtils.MounterUtils) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Path string `json:"path"`
		}

		if err := c.BindJSON(&request); err != nil {
			logger.Error("invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		logger.Info("New unmount request with values: ", zap.String("Path", request.Path))

		err := mounter.FuseUnmount(request.Path)
		if err != nil {
			logger.Error("unmount failed: ", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("unmount failed :%v", err)})
			return
		}

		logger.Info("bucket unmount is successful", zap.Any("path", request.Path))
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
