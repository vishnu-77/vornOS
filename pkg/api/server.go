// Package api implements the gRPC server for the Vorn Runtime Manager.
package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"

	"github.com/vornos/buildos/pkg/config"
	"github.com/vornos/buildos/pkg/gpu"
	"github.com/vornos/buildos/pkg/jobs"
	"github.com/vornos/buildos/pkg/logging"
	"github.com/vornos/buildos/pkg/models"
	"github.com/vornos/buildos/pkg/runtime"
	"github.com/vornos/buildos/pkg/scheduler"
	"github.com/vornos/buildos/pkg/store"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Server wraps the gRPC server and all subsystem references.
type Server struct {
	grpcServer *grpc.Server
	cfg        *config.Config
	handlers   *Handlers
	log        *zap.SugaredLogger
}

// Deps bundles all subsystem dependencies needed by the API server.
type Deps struct {
	Config       *config.Config
	Store        store.Store
	RuntimeStore *runtime.Store
	ModelStore   *models.Store
	JobManager   *jobs.Manager
	Scheduler    *scheduler.Scheduler
	GPUInventory *gpu.Inventory
	AuditLogger  *logging.AuditLogger
	Logger       *zap.SugaredLogger
}

// NewServer creates a gRPC server with all handlers registered.
func NewServer(deps *Deps) (*Server, error) {
	var opts []grpc.ServerOption

	// mTLS if configured
	if deps.Config.TLSEnabled() {
		tlsCfg, err := loadTLSConfig(deps.Config)
		if err != nil {
			return nil, fmt.Errorf("tls config: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	gs := grpc.NewServer(opts...)
	h := NewHandlers(deps)

	// TODO: register VornService with the generated proto server
	// vornpb.RegisterVornServiceServer(gs, h)
	_ = h

	return &Server{
		grpcServer: gs,
		cfg:        deps.Config,
		handlers:   h,
		log:        deps.Logger,
	}, nil
}

// ServeTCP starts the gRPC server on the configured TCP address.
func (s *Server) ServeTCP(lis net.Listener) error {
	s.log.Infow("gRPC server starting (TCP)", "addr", lis.Addr())
	return s.grpcServer.Serve(lis)
}

// ServeUnix starts the gRPC server on a Unix domain socket.
func (s *Server) ServeUnix(path string) error {
	// Remove stale socket
	os.Remove(path)

	lis, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", path, err)
	}

	// Set socket permissions: only vorn group can connect
	if err := os.Chmod(path, 0660); err != nil {
		lis.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}

	s.log.Infow("gRPC server starting (Unix)", "path", path)
	return s.grpcServer.Serve(lis)
}

// GracefulStop initiates a graceful shutdown of the gRPC server.
func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}

// loadTLSConfig builds a TLS config for mutual TLS.
func loadTLSConfig(cfg *config.Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("load server cert: %w", err)
	}

	caCert, err := os.ReadFile(cfg.TLSCA)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}
