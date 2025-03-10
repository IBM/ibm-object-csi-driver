package main

import (
	"flag"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
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
)

// SystemOperation is an interface for system operations like mount and unmount.
type SystemOperation interface {
	Execute(command string, args ...string) (string, error)
}

// RealSystemOperation is an implementation of SystemOperation that performs actual system operations.
type RealSystemOperation struct{}

func (rs *RealSystemOperation) Execute(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	output, err := cmd.CombinedOutput()
	return string(output), err
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

	logger.Info("Starting mount-helper-container service...")

	// Create gin router
	router := gin.Default()

	// Add REST APIs to router
	router.POST("/api/cos/mount", handleCosMount())
	router.POST("/api/cos/unmount", handleCosUnmount())

	// Serve HTTP requests over Unix socket
	err = http.Serve(listener, router)
	if err != nil {
		logger.Fatal("Error while serving HTTP requests:", zap.Error(err))
	}
}

func mountHelperContainerStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"Message": "Mount-helper-container server is live!"})
}

// handleMounting mounts ibmshare based file system mountPath to targetPath
func handleMounting(sysOp SystemOperation) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			MountPath  string `json:"mountPath"`
			TargetPath string `json:"targetPath"`
			FsType     string `json:"fsType"`
			RequestID  string `json:"requestID"`
		}

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New mount request with values: ", zap.String("RequestID:", request.RequestID), zap.String("Source mount Path:", request.MountPath), zap.String("Target Path:", request.TargetPath))

		// execute mount command
		options := "mount -t " + request.FsType + " -o secure=true " + request.MountPath + " " + request.TargetPath + " -v"

		logger.Info("Command to execute is: ", zap.String("Command:", options))

		output, err := sysOp.Execute("mount", "-t", request.FsType, "-o", "secure=true", request.MountPath, request.TargetPath, "-v")
		if err != nil {
			logger.Error("Mounting failed with error: ", zap.Error(err))
			logger.Error("Command output: ", zap.String("output", output))
			response := gin.H{
				"MountExitCode": err.Error(),
				"Description":   output,
			}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		logger.Info("Command output: ", zap.String("", output))
		c.JSON(http.StatusOK, gin.H{"message": "Request processed successfully"})
	}
}

// handleUnMount does umount on a targetPath provided
func handleUnMount(sysOp SystemOperation) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			TargetPath string `json:"targetPath"`
		}

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New umount request with values: ", zap.String("Target Path:", request.TargetPath))

		output, err := sysOp.Execute("umount", request.TargetPath)

		if err != nil {
			logger.Error("Umount failed with error: ", zap.Error(err))
			logger.Error("Command output: ", zap.String("output", output))
			response := gin.H{
				"MountExitCode": err.Error(),
				"Description":   output,
			}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		c.JSON(http.StatusOK, gin.H{"Message": "Request processed successfully"})
	}
}

// debugLogs collectes logs necessary in case there are any mount failures.
func debugLogs(sysOp SystemOperation) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			RequestID string `json:"requestID"`
		}

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		output, err := sysOp.Execute("journalctl", "-u", "mount-helper-container")

		if err != nil {
			logger.Error("Unable to fetch logs, error: ", zap.Error(err))
			logger.Error("Command output: ", zap.String("output", output))
			response := gin.H{
				"MountExitCode": err.Error(),
				"Description":   output,
			}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		logFile, err := os.Create("/tmp/mount-helper-container.log")
		if err != nil {
			logger.Error("Not able to create log file: ", zap.Error(err))
			return
		}
		defer logFile.Close()

		// Write the output to the file
		_, err = logFile.WriteString(string(output))
		if err != nil {
			logger.Error("Error writing to file:", zap.Error(err))
			return
		}

		// mount-helper logs are stored at /opt/ibm/mount-ibmshare/mount-ibmshare.log. Available at volume mount path

		c.JSON(http.StatusOK, gin.H{"Message": "Request processed successfully"})
	}
}

// mountStatus takes target directory as input and checks if it is a valid mount directory
func mountStatus(sysOp SystemOperation) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			TargetPath string `json:"targetPath"`
		}

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New find mount request with values: ", zap.String("Target Path:", request.TargetPath))

		output, err := sysOp.Execute("findmnt", request.TargetPath)

		if err != nil {
			logger.Error("'findmnt' failed with error: ", zap.Error(err))
			response := gin.H{
				"MountExitCode": err.Error(),
				"Description":   output,
			}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		c.JSON(http.StatusOK, "Success!!")
	}
}

func handleCosMount() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Path    string   `json:"path"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}

		logger.Info("New mount request with values: ", zap.Any("Request:", request))

		if err := c.BindJSON(&request); err != nil {
			logger.Error("Invalid request: ", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		logger.Info("New mount request with values: ", zap.String("Path:", request.Path), zap.String("Command:", request.Command), zap.Any("Args:", request.Args))

		utils := mounterUtils.MounterOptsUtils{}
		err := utils.FuseMount(request.Path, request.Command, request.Args)
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
