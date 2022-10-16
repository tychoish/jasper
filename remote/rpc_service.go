package remote

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/recovery"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/remote/internal"
	"github.com/tychoish/jasper/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// AttachService attaches the jasper GRPC server to the given manager. After
// this function successfully returns, calls to Manager functions will be sent
// over GRPC to the Jasper GRPC server.
func AttachService(ctx context.Context, manager jasper.Manager, s *grpc.Server) error {
	return errors.WithStack(internal.AttachService(ctx, manager, s))
}

// StartRPCService starts an RPC server with the specified address addr around the
// given manager. If creds is non-nil, the credentials will be used to establish
// a secure TLS connection with clients; otherwise, it will start an insecure
// service. The caller is responsible for closing the connection using the
// returned jasper.CloseFunc.
//
// This service does not have any kind of interceptors (middleware) or
// logging configured, and panics are not handled. Passing
// interceptors from the aviation package or grpc-middleware as
// gprc.ServerOptions to this function can handle that.
func StartRPCService(ctx context.Context, manager jasper.Manager, addr net.Addr, creds *options.CertificateCredentials, opts ...grpc.ServerOption) (util.CloseFunc, error) {
	lis, err := net.Listen(addr.Network(), addr.String())
	if err != nil {
		return nil, errors.Wrapf(err, "error listening on %s", addr.String())
	}

	if creds != nil {
		tlsConf, err := creds.Resolve()
		if err != nil {
			return nil, fmt.Errorf("error generating TLS config from server credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConf)))
	}

	service := grpc.NewServer(opts...)

	ctx, cancel := context.WithCancel(ctx)
	if err := AttachService(ctx, manager, service); err != nil {
		cancel()
		return nil, fmt.Errorf("could not attach manager to service: %w", err)
	}
	go func() {
		defer recovery.LogStackTraceAndContinue("RPC service")
		grip.Notice(service.Serve(lis))
	}()

	return func() error { service.Stop(); cancel(); return nil }, nil
}

// StartRPCServiceWithFile is the same as StartService, but the credentials will be
// read from the file given by filePath if the filePath is non-empty. The
// credentials file should contain the JSON-encoded bytes from
// (*Credentials).Export().
func StartRPCServiceWithFile(ctx context.Context, manager jasper.Manager, addr net.Addr, filePath string, opts ...grpc.ServerOption) (util.CloseFunc, error) {
	var creds *options.CertificateCredentials
	if filePath != "" {
		var err error
		creds, err = options.NewCredentialsFromFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error getting credentials from file: %w", err)
		}
	}

	return StartRPCService(ctx, manager, addr, creds, opts...)
}
