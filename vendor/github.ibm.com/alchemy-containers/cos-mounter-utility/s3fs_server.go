package main

/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/


import (
	"context"
	"fmt"
	"flag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"log"
	"os"
	"os/exec"
	"syscall"
	"github.ibm.com/alchemy-containers/cos-mounter-utility/s3fs"
	"google.golang.org/grpc"
)

const (
	githubName     = "ibm-object-csi-driver"
	pluginLogName  = "IBM Object Storage CSI Driver"
)

var (
	logger   *zap.Logger
	endpoint = flag.String("endpoint", "/tmp/mounter.sock", "Mounter endpoint")
)

// server implements UnimplementedS3FSServiceServer
type server struct{
	s3fs.UnimplementedS3FSServiceServer
}

func (s *server) Mount(ctx context.Context, req *s3fs.MountRequest) (*s3fs.MountResponse, error) {
	// Execute the s3fs command with the provided arguments
	cmd := exec.Command("s3fs", req.Args...)

	err := cmd.Run()
	if err != nil {
		return &s3fs.MountResponse{Success: false, Message: fmt.Sprintf("Failed to mount S3FS: %v", err)}, err
	}

	return &s3fs.MountResponse{Success: true, Message: "S3FS mounted successfully"}, nil
}

func (s *server) Unmount(ctx context.Context, req *s3fs.UnmountRequest) (*s3fs.UnmountResponse, error) {
	// Use syscall.Unmount to unmount the S3 bucket
	err := syscall.Unmount(req.MountPoint, syscall.MNT_DETACH)
	if err != nil {
		return &s3fs.UnmountResponse{Success: false, Message: fmt.Sprintf("Failed to unmount S3FS: %v", err)}, err
	}

	return &s3fs.UnmountResponse{Success: true, Message: "S3FS unmounted successfully"}, nil
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
	), zap.AddCaller()).With(zap.String("Name", githubName)).With(zap.String("DriverName", pluginLogName))
	atom.SetLevel(zap.InfoLevel)
	return logger
}

func main() {
	var err error
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	flag.Parse()
	logger = setUpLogger()
	defer logger.Sync()
	lis, err := net.Listen("unix", *endpoint)
	if err != nil {
		logger.Fatal("COS CSI Mounter failed to listen", zap.Error(err))
	}
	//defer os.Remove(*endpoint) // Remove the socket file on program exiti
	s := grpc.NewServer()
	s3fs.RegisterS3FSServiceServer(s, &server{})

	log.Println("Server started on port 50051")
	if err := s.Serve(lis); err != nil {
		logger.Fatal("Failed to start rpc server", zap.Error(err))
	}
}
