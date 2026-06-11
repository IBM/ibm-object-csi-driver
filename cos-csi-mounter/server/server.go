//go:build linux
// +build linux

package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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

	Version   = "dev"
	GitCommit = "none"
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	logger = setUpLogger()
	defer func() {
		if err := logger.Sync(); err != nil && !isInvalidSync(err) {
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
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller()).With(zap.String("ServiceName", "cos-csi-mounter"))
	atom.SetLevel(zap.InfoLevel)
	return logger
}

func isInvalidSync(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "invalid argument") ||
		strings.Contains(strings.ToLower(err.Error()), "inappropriate ioctl") // catch edge cases on some platforms
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
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("Version: %s\nGit Commit: %s\n", Version, GitCommit)
		return
	}
	err := startService(setupSocket, newRouter(), handleSignals)
	if err != nil {
		logger.Error("cos-csi-mounter exited with error", zap.Error(err))
		os.Exit(1)
	}
}

func handleCosMount(mounter mounterUtils.MounterUtils, parser MounterArgsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract request ID from HTTP header
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = "unknown"
		}
		log := logger.With(zap.String("request_id", reqID))

		var request MountRequest

		if err := c.BindJSON(&request); err != nil {
			log.Error("Invalid request", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("[%s] invalid request", reqID)})
			return
		}

		log.Info("New mount request",
			zap.String("bucket", request.Bucket),
			zap.String("path", request.Path),
			zap.String("mounter", request.Mounter),
			zap.Any("args", request.Args))

		if request.Mounter != constants.S3FS && request.Mounter != constants.RClone {
			log.Error("Invalid mounter", zap.String("mounter", request.Mounter))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("[%s] invalid mounter", reqID)})
			return
		}

		if request.Bucket == "" {
			log.Error("Missing bucket in request")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("[%s] missing bucket", reqID)})
			return
		}

		// validate mounter args
		log.Debug("Parsing mounter args")
		args, err := parser.Parse(request)
		if err != nil {
			log.Error("Failed to parse mounter args",
				zap.String("mounter", request.Mounter), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("[%s] invalid args for mounter: %v", reqID, err)})
			return
		}

		log.Info("Mounting bucket",
			zap.String("path", request.Path),
			zap.String("mounter", request.Mounter))
		err = mounter.FuseMount(request.Path, request.Mounter, args)
		if err != nil {
			log.Error("Mount failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("[%s] mount failed: %v", reqID, err)})
			return
		}

		log.Info("Bucket mount successful",
			zap.String("bucket", request.Bucket),
			zap.String("path", request.Path))
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func handleCosUnmount(mounter mounterUtils.MounterUtils) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract request ID from HTTP header
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = "unknown"
		}
		log := logger.With(zap.String("request_id", reqID))

		var request struct {
			Path string `json:"path"`
		}

		if err := c.BindJSON(&request); err != nil {
			log.Error("Invalid request", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("[%s] invalid request", reqID)})
			return
		}

		log.Info("New unmount request", zap.String("path", request.Path))

		log.Info("Unmounting bucket", zap.String("path", request.Path))
		err := mounter.FuseUnmount(request.Path)
		if err != nil {
			log.Error("Unmount failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("[%s] unmount failed: %v", reqID, err)})
			return
		}

		log.Info("Bucket unmount successful", zap.String("path", request.Path))
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
