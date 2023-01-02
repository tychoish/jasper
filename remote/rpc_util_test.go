package remote

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/tychoish/emt"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func makeInsecureRPCServiceAndClient(ctx context.Context, mngr jasper.Manager) (Manager, error) {
	addr, err := tryStartRPCService(ctx, func(ctx context.Context, addr net.Addr) error {
		return startTestRPCService(ctx, mngr, addr, nil)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start RPC service: %w", err)
	}
	return newTestRPCClient(ctx, addr, nil)
}

func tryStartRPCService(ctx context.Context, startService func(context.Context, net.Addr) error) (net.Addr, error) {
	var addr net.Addr
	var err error
tryPort:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break tryPort
		default:
			addr, err = net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", testutil.GetPortNumber()))
			if err != nil {
				continue
			}

			if err = startService(ctx, addr); err != nil {
				continue
			}

			break tryPort
		}
	}
	return addr, err
}

func makeTLSRPCServiceAndClient(ctx context.Context, mngr jasper.Manager) (Manager, error) {
	caCertFile := filepath.Join("testdata", "ca.crt")

	serverCertFile := filepath.Join("testdata", "server.crt")
	serverKeyFile := filepath.Join("testdata", "server.key")

	clientCertFile := filepath.Join("testdata", "client.crt")
	clientKeyFile := filepath.Join("testdata", "client.key")

	// Make CA credentials
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %w", err)
	}

	// Make server credentials
	serverCert, err := os.ReadFile(serverCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %w", err)
	}
	serverKey, err := os.ReadFile(serverKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	serverCreds, err := options.NewCredentials(caCert, serverCert, serverKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize test server credentials: %w", err)
	}

	addr, err := tryStartRPCService(ctx, func(ctx context.Context, addr net.Addr) error {
		return startTestRPCService(ctx, mngr, addr, serverCreds)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start RPC service: %w", err)
	}

	clientCert, err := os.ReadFile(clientCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %w", err)
	}
	clientKey, err := os.ReadFile(clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	clientCreds, err := options.NewCredentials(caCert, clientCert, clientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize test client credentials: %w", err)
	}

	return newTestRPCClient(ctx, addr, clientCreds)
}

// startTestService creates a server for testing purposes that terminates when
// the context is done.
func startTestRPCService(ctx context.Context, mngr jasper.Manager, addr net.Addr, creds *options.CertificateCredentials) error {
	closeService, err := StartRPCService(ctx, mngr, addr, creds)
	if err != nil {
		return fmt.Errorf("could not start server: %w", err)
	}

	go func() {
		<-ctx.Done()
		grip.Error(closeService())
	}()

	return nil
}

// newTestClient establishes a client for testing purposes that closes when
// the context is done.
func newTestRPCClient(ctx context.Context, addr net.Addr, creds *options.CertificateCredentials) (Manager, error) {
	client, err := NewRPCClient(ctx, addr, creds)
	if err != nil {
		return nil, fmt.Errorf("could not get client: %w", err)
	}

	go func() {
		<-ctx.Done()
		grip.Notice(client.CloseConnection())
	}()

	return client, nil
}

func createProcs(ctx context.Context, opts *options.Create, manager jasper.Manager, num int) ([]jasper.Process, error) {
	catcher := emt.NewBasicCatcher()
	out := []jasper.Process{}
	for i := 0; i < num; i++ {
		optsCopy := *opts

		proc, err := manager.CreateProcess(ctx, &optsCopy)
		catcher.Add(err)
		if proc != nil {
			out = append(out, proc)
		}
	}

	return out, catcher.Resolve()
}
