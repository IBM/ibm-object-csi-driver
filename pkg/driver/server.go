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
	"errors"
	"net"
	"net/url"
	"os"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
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

// nonBlockingGRPCServer server
type nonBlockingGRPCServer struct {
	wg     sync.WaitGroup
	server *grpc.Server
	mode   string
	logger *zap.Logger
}

// NewNonBlockingGRPCServer ...
func NewNonBlockingGRPCServer(mode string, logger *zap.Logger) NonBlockingGRPCServer {
	return &nonBlockingGRPCServer{mode: mode, logger: logger}
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
	s.logger.Info("nonBlockingGRPCServer-Setup", zap.Reflect("Endpoint", endpoint))

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logGRPC),
	}

	u, err := url.Parse(endpoint)

	if err != nil {
		msg := "failed to parse endpoint"
		s.logger.Error(msg, zap.Error(err))
		return nil, err
	}

	var addr string
	switch u.Scheme {
	case "unix":
		addr = u.Path
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			s.logger.Error("failed to remove", zap.Reflect("addr", addr), zap.Error(err))
			return nil, err
		}
	case "tcp":
		addr = u.Host
	default:
		msg := "endpoint scheme not supported"
		s.logger.Error(msg, zap.Reflect("Scheme", u.Scheme))
		return nil, errors.New(msg)
	}

	s.logger.Info("start listening GRPC server", zap.Reflect("Scheme", u.Scheme), zap.Reflect("Addr", addr))

	listener, err := net.Listen(u.Scheme, addr)
	if err != nil {
		msg := "failed to listen GRPC server"
		s.logger.Error(msg, zap.Reflect("Error", err))
		return nil, errors.New(msg)
	}

	server := grpc.NewServer(opts...)
	s.server = server

	csi.RegisterIdentityServer(s.server, ids)

	switch s.mode {
	case "controller":
		klog.V(3).Info("--Starting server in controller mode--")
		csi.RegisterControllerServer(s.server, cs)
	case "node":
		klog.V(3).Info("--Starting server in node server mode--")
		csi.RegisterNodeServer(s.server, ns)
	case "controller-node":
		klog.V(3).Info("--Starting node and controller server mode--")
		csi.RegisterControllerServer(s.server, cs)
		csi.RegisterNodeServer(s.server, ns)
	}

	return listener, nil
}

// serve ...
func (s *nonBlockingGRPCServer) serve(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	s.logger.Info("nonBlockingGRPCServer-server", zap.Reflect("Endpoint", endpoint))
	//! Setup
	listener, err := s.Setup(endpoint, ids, cs, ns)
	if err != nil {
		s.logger.Fatal("failed to setup GRPC Server", zap.Error(err))
	}
	s.logger.Info("listening GRPC server for connections", zap.Reflect("Addr", listener.Addr().String()))
	if err := s.server.Serve(listener); err != nil {
		s.logger.Info("failed to serve", zap.Error(err))
	}
}

// logGRPC ...
func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	klog.V(3).Infof("GRPC call: %s", info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("GRPC error: %v", err)
	} else {
		klog.V(5).Infof("GRPC response: %+v", resp)
	}
	return resp, err
}
