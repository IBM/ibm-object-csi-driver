/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2019, 2020 All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

package driver

import (
	"errors"
	"net"
	"net/url"
	"os"
	"sync"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

// NonBlockingGRPCServer Defines Non blocking GRPC server interfaces
type NonBlockingGRPCServer interface {
	// Start services at the endpoint
	Start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer)
	// Waits for the service to stop
	Wait()
	// Stops the service gracefully
	Stop()
	// Stops the service forcefully
	ForceStop()
}

// NewNonBlockingGRPCServer ...
func NewNonBlockingGRPCServer(logger *zap.Logger) NonBlockingGRPCServer {
	return &nonBlockingGRPCServer{logger: logger}
}

// nonBlockingGRPCServer server
type nonBlockingGRPCServer struct {
	wg     sync.WaitGroup
	server *grpc.Server
	logger *zap.Logger
}

// Start ...
func (s *nonBlockingGRPCServer) Start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	s.wg.Add(1)

	go s.serve(endpoint, ids, cs, ns)
}

// Wait ...
func (s *nonBlockingGRPCServer) Wait() {
	s.wg.Wait()
}

// Stop ...
func (s *nonBlockingGRPCServer) Stop() {
	s.server.GracefulStop()
}

// ForceStop ...
func (s *nonBlockingGRPCServer) ForceStop() {
	s.server.Stop()
}

// Setup ...
func (s *nonBlockingGRPCServer) Setup(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) (net.Listener, error) {
	s.logger.Info("nonBlockingGRPCServer-Setup...", zap.Reflect("Endpoint", endpoint))

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logGRPC),
	}

	u, err := url.Parse(endpoint)

	if err != nil {
		msg := "Failed to parse endpoint"
		s.logger.Error(msg, zap.Error(err))
		return nil, err
	}

	var addr string
	if u.Scheme == "unix" {
		addr = u.Path
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			s.logger.Error("Failed to remove", zap.Reflect("addr", addr), zap.Error(err))
			return nil, err
		}
	} else if u.Scheme == "tcp" {
		addr = u.Host
	} else {
		msg := "Endpoint scheme not supported"
		s.logger.Error(msg, zap.Reflect("Scheme", u.Scheme))
		return nil, errors.New(msg)
	}

	s.logger.Info("Start listening GRPC Server", zap.Reflect("Scheme", u.Scheme), zap.Reflect("Addr", addr))

	listener, err := net.Listen(u.Scheme, addr)
	if err != nil {
		msg := "Failed to listen GRPC Server"
		s.logger.Error(msg, zap.Reflect("Error", err))
		return nil, errors.New(msg)
	}

	server := grpc.NewServer(opts...)
	s.server = server

	if ids != nil {
		csi.RegisterIdentityServer(s.server, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(s.server, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(s.server, ns)
	}
	return listener, nil
}

// serve ...
func (s *nonBlockingGRPCServer) serve(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	s.logger.Info("nonBlockingGRPCServer-serve...", zap.Reflect("Endpoint", endpoint))
	//! Setup
	listener, err := s.Setup(endpoint, ids, cs, ns)
	if err != nil {
		s.logger.Fatal("Failed to setup GRPC Server", zap.Error(err))
	}
	s.logger.Info("Listening GRPC server for connections", zap.Reflect("Addr", listener.Addr()))
	if err := s.server.Serve(listener); err != nil {
		s.logger.Info("Failed to serve", zap.Error(err))
	}
}

// logGRPC ...
func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	klog.V(3).Infof("GRPC call: %s", info.FullMethod)
	klog.V(5).Infof("GRPC request: %+v", req)
	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("GRPC error: %v", err)
	} else {
		klog.V(5).Infof("GRPC response: %+v", resp)
	}
	return resp, err
}
