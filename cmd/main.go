/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

//Package main ...
package main

import (
	"flag"
	libMetrics "github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	csiConfig "github.com/IBM/satellite-object-storage-plugin/config"
	driver "github.com/IBM/satellite-object-storage-plugin/pkg/driver"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/utils"
	//"github.com/ctrox/csi-s3/pkg/s3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.ibm.com/alchemy-containers/ibm-csi-common/pkg/metrics"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var (
	endpoint       = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	nodeID         = flag.String("nodeid", "", "node id")
	fileLogger     *zap.Logger
	logfile        = flag.String("log", "", "log file")
	metricsAddress = flag.String("metrics-address", "0.0.0.0:9080", "Metrics address")
	vendorVersion  string
)

func getFromEnv(key string, defaultVal string) string {
	value := os.Getenv(key)
	if value == "" && defaultVal != "" {
		value = defaultVal
	} else {
		value = "/var/log/satellite-obj-storage.log"
	}
	return value
}

func getZapLogger() *zap.Logger {
	logfilepath := getFromEnv("SATOBJLOGFILE", *logfile)

	lumberjackLogger := &lumberjack.Logger{
		Filename:   logfilepath,
		MaxSize:    100, //MB
		MaxBackups: 10,  //Maximum number of backup
		MaxAge:     60,  //Days
	}

	prodConf := zap.NewProductionEncoderConfig()
	prodConf.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(prodConf)

	zapsync := zapcore.AddSync(lumberjackLogger)

	loglevel := zap.NewAtomicLevelAt(zapcore.InfoLevel)

	loggercore := zapcore.NewCore(encoder, zapsync, loglevel)

	logger := zap.New(loggercore)
	logger.Named("SatelliteObjStoragePlugin")

	return logger
}

func init() {
	flag.Set("logtostderr", "true")
	fileLogger = getZapLogger()
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	handle(fileLogger)
	os.Exit(0)
}

func handle(logger *zap.Logger) {
	if vendorVersion == "" {
		logger.Fatal("CSI driver vendorVersion must be set at compile time")
	}
	logger.Info("S3 driver version", zap.Reflect("DriverVersion", vendorVersion))
	logger.Info("Controller Mutex Lock enabled", zap.Bool("LockEnabled", *utils.LockEnabled))

	csiDriver, err := driver.Setups3Driver(logger, csiConfig.CSIPluginGithubName, vendorVersion)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	S3CSIDriver, err := csiDriver.NewS3CosDriver(*nodeID, *endpoint)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	serveMetrics()
	S3CSIDriver.Run()
}

func serveMetrics() {
	fileLogger.Info("Starting metrics endpoint")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		//http.Handle("/health-check", healthCheck)
		err := http.ListenAndServe(*metricsAddress, nil)
		fileLogger.Error("Failed to start metrics service:", zap.Error(err))
	}()
	metrics.RegisterAll(csiConfig.CSIPluginGithubName)
	libMetrics.RegisterAll()
}
