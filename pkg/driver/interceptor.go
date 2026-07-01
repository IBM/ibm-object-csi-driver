/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

import (
	"context"
	"time"

	"github.com/IBM/ibm-object-csi-driver/pkg/requestid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// UnaryServerInterceptor adds request ID to all unary RPC calls and logs request lifecycle
func UnaryServerInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Generate or extract request ID
		ctx, reqID := requestid.GetOrGenerate(ctx)
		
		// Log request start
		startTime := time.Now()
		logger.Info("gRPC request started",
			zap.String("request_id", reqID),
			zap.String("method", info.FullMethod),
			zap.Time("start_time", startTime))
		
		// Call the handler
		resp, err := handler(ctx, req)
		
		// Log request completion
		duration := time.Since(startTime)
		if err != nil {
			logger.Error("gRPC request failed",
				zap.String("request_id", reqID),
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.Error(err))
		} else {
			logger.Info("gRPC request completed",
				zap.String("request_id", reqID),
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration))
		}
		
		return resp, err
	}
}

