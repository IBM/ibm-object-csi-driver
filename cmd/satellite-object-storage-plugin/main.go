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

// Package main ...
package main

import (
	"flag"
	libMetrics "github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	csiConfig "github.com/IBM/satellite-object-storage-plugin/config"
	driver "github.com/IBM/satellite-object-storage-plugin/pkg/driver"
	"github.com/IBM/satellite-object-storage-plugin/pkg/mounter"
	"github.com/IBM/satellite-object-storage-plugin/pkg/s3client"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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
	flag.Set("logtostderr", "true") //nolint
	flag.Parse()
	return &Options{
		ServerMode:     *serverMode,
		Endpoint:       *endpoint,
		NodeID:         *nodeID,
		MetricsAddress: *metricsAddress,
	}
}

func getZapLogger() *zap.Logger {
	prodConf := zap.NewProductionConfig()
	prodConf.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := prodConf.Build()
	logger.Named("SatelliteObjStoragePlugin")

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
	csiDriver, err := driver.Setups3Driver(options.ServerMode, csiConfig.CSIDriverName, csiConfig.VendorVersion, logger)
	if err != nil {
		logger.Fatal("Failed to setup s3 driver", zap.Error(err))
		os.Exit(1)
	}
	S3CSIDriver, err := csiDriver.NewS3CosDriver(options.NodeID, options.Endpoint, s3client.NewObjectStorageSessionFactory(), mounter.NewS3fsMounterFactory())
	if err != nil {
		logger.Fatal("Failed in initialize s3 COS driver", zap.Error(err))
		os.Exit(1)
	}
	serveMetrics(options.MetricsAddress, logger)
	S3CSIDriver.Run()
}

func serveMetrics(metricsAddress string, logger *zap.Logger) {
	logger.Info("starting metrics endpoint")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		//http.Handle("/health-check", healthCheck)
		err := http.ListenAndServe(metricsAddress, nil)
		logger.Error("failed to start metrics service:", zap.Error(err))
	}()
	// TODO
	//metrics.RegisterAll(csiConfig.CSIPluginGithubName)
	libMetrics.RegisterAll()
}
