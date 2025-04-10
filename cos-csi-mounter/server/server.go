package main

import (
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	logger.Info("Starting cos-csi-mounter service...")

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
			Path    string   `json:"path"`
			Mounter string   `json:"mounter"`
			Args    []string `json:"args"`
		}

		logger.Info("New mount request with values: ", zap.Any("Request:", request))

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New mount request with values: ", zap.String("Path:", request.Path), zap.String("Mounter:", request.Mounter), zap.Any("Args:", request.Args))

		if request.Mounter != s3fs && request.Mounter != rclone {
			logger.Error("Invalid Request!!!!")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		utils := mounterUtils.MounterOptsUtils{}
		err := utils.FuseMount(request.Path, request.Mounter, request.Args)
		if err != nil {
			logger.Error("Mount Failed: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mount Failed"})
			return
		}

		logger.Info("New COS Mount is Successfull")
		c.JSON(http.StatusOK, "Success!!")
	}
}

// Currently by calling FuseUnmount method from NodeServer is able to unmount the volume
// If required to have specific unmount methods based on mounters then following call can be modified and used

func handleCosUnmount() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Path string `json:"path"`
		}

		logger.Info("New unmount request with values: ", zap.Any("Request:", request))

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New unmount request with values: ", zap.String("Path:", request.Path))

		utils := mounterUtils.MounterOptsUtils{}
		err := utils.FuseUnmount(request.Path)
		if err != nil {
			logger.Error("UnMount Failed: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mount Failed"})
			return
		}

		logger.Info("COS UnMount is Successfull")
		c.JSON(http.StatusOK, "Success!!")
	}
}
