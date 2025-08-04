/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package main ...
package main

import (
	"flag"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/ibm-object-csi-driver/config"
	"github.com/IBM/ibm-object-csi-driver/pkg/constants"
	"github.com/IBM/ibm-object-csi-driver/pkg/driver"
	"github.com/IBM/ibm-object-csi-driver/pkg/mounter"
	mounterUtils "github.com/IBM/ibm-object-csi-driver/pkg/mounter/utils"
	"github.com/IBM/ibm-object-csi-driver/pkg/s3client"
	"github.com/IBM/ibm-object-csi-driver/pkg/utils"
	libMetrics "github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
)

// Options is the combined set of options for all operating modes.
type Options struct {
	ServerMode     string
	Endpoint       string
	NodeID         string
	MetricsAddress string
}

func getOptions() *Options {
	var (
		endpoint       = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
		serverMode     = flag.String("servermode", "controller", "Server Mode node/controller")
		nodeID         = flag.String("nodeid", "host01", "node id")
		metricsAddress = flag.String("metrics-address", "0.0.0.0:9080", "Metrics address")
	)
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis.Error cannot be usefully handled.
	flag.Parse()
	return &Options{
		ServerMode:     *serverMode,
		Endpoint:       *endpoint,
		NodeID:         *nodeID,
		MetricsAddress: *metricsAddress,
	}
}

func getZapLogger() *zap.Logger {
	// Prepare a new logger
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller()).With(zap.String("name", config.CSIPluginGithubName)).With(zap.String("CSIDriverName", "IBM CSI Object Driver"))

	atom.SetLevel(zap.InfoLevel)
	return logger
}

func getEnv(key string) string {
	return os.Getenv(strings.ToUpper(key))
}

func getConfigBool(envKey string, defaultConf bool, logger zap.Logger) bool {
	if val := getEnv(envKey); val != "" {
		if envBool, err := strconv.ParseBool(val); err == nil {
			return envBool
		}
		logger.Error("error parsing env val to bool", zap.String("env", envKey))
	}
	return defaultConf
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	logger := getZapLogger()
	loggerLevel := zap.NewAtomicLevel()
	options := getOptions()

	klog.V(1).Info("Starting Server...")

	debugTrace := getConfigBool("DEBUG_TRACE", false, *logger)
	if debugTrace {
		loggerLevel.SetLevel(zap.DebugLevel)
	}

	serverSetup(options, logger)
	os.Exit(0)
}

func serverSetup(options *Options, logger *zap.Logger) {
	csiDriver, err := driver.Setups3Driver(options.ServerMode, config.CSIDriverName, config.VendorVersion, logger)
	if err != nil {
		logger.Fatal("Failed to setup s3 driver", zap.Error(err))
		os.Exit(1)
	}

	statsUtil := &(utils.DriverStatsUtils{})
	mounterUtil := &(mounterUtils.MounterOptsUtils{})

	S3CSIDriver, err := csiDriver.NewS3CosDriver(options.NodeID, options.Endpoint, s3client.NewObjectStorageSessionFactory(), mounter.NewCSIMounterFactory(), statsUtil, mounterUtil)
	if err != nil {
		logger.Fatal("Failed in initialize s3 COS driver", zap.Error(err))
		os.Exit(1)
	}
	serveMetrics(options.ServerMode, options.MetricsAddress, logger)
	S3CSIDriver.Run()
}

func serveMetrics(mode, metricsAddress string, logger *zap.Logger) {
	logMsg := "starting metrics endpoint"
	if strings.Contains(mode, "node") {
		logMsg = "starting metrics & cos-csi-mounter socket-health endpoints"
	}
	logger.Info(logMsg)
	http.Handle("/metrics", promhttp.Handler())
	if strings.Contains(mode, "node") {
		http.HandleFunc("/cos-csi-mounter/socket-health", func(w http.ResponseWriter, _ *http.Request) {
			if err := checkCosCsiMounterSocketHealth(); err != nil {
				http.Error(w, "unhealthy: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
	}

	go func() {
		// #nosec G114 -- use default timeout.
		if err := http.ListenAndServe(metricsAddress, nil); err != nil {
			logMsg = "failed to start metrics service:"
			if strings.Contains(mode, "node") {
				logMsg = "failed to start metrics & cos-csi-mounter socket-health service:"
			}
			logger.Error(logMsg, zap.String("addr", metricsAddress), zap.Error(err))
		}
	}()
	// TODO
	//metrics.RegisterAll(csiConfig.CSIPluginGithubName)
	libMetrics.RegisterAll()
}

func checkCosCsiMounterSocketHealth() error {
	socketPath := os.Getenv(constants.COSCSIMounterSocketPathEnv)
	if socketPath == "" {
		socketPath = constants.COSCSIMounterSocketPath
	}

	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return err
	}
	err = conn.Close()
	if err != nil {
		return err
	}
	return nil
}
