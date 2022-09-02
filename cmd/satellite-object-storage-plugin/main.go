/**
 * Copyright 2021 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//Package main ...
package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	libMetrics "github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	csiConfig "github.com/IBM/satellite-object-storage-plugin/config"
	driver "github.com/IBM/satellite-object-storage-plugin/pkg/driver"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	endpoint       = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	nodeID         = flag.String("nodeid", "", "node id")
	fileLogger     *zap.Logger
	logfile        = flag.String("log", "", "log file")
	metricsAddress = flag.String("metrics-address", "0.0.0.0:9080", "Metrics address")
	// vendorVersion  string
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
	// if *vendorVersion == "" {
	// 	logger.Fatal("CSI driver vendorVersion must be set at compile time")
	// }
	// logger.Info("S3 driver version", zap.Reflect("DriverVersion", vendorVersion))
	// TODO
	//logger.Info("Controller Mutex Lock enabled", zap.Bool("LockEnabled", *utils.LockEnabled))

	csiDriver, err := driver.Setups3Driver(logger, csiConfig.CSIPluginGithubName, csiConfig.VendorVersion)
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
	// TODO
	//metrics.RegisterAll(csiConfig.CSIPluginGithubName)
	libMetrics.RegisterAll()
}
