package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/evergreen-ci/service"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/jasper/x/remote"
)

func TestDaemon(t *testing.T) {
	for daemonAndClientName, makeDaemonAndClient := range map[string]func(ctx context.Context, t *testing.T, manager jasper.Manager) (util.CloseFunc, remote.Manager){
		"RPCService": func(ctx context.Context, t *testing.T, _ jasper.Manager) (util.CloseFunc, remote.Manager) {
			port := testutil.GetPortNumber()
			manager := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

			daemon := newRPCDaemon("localhost", port, manager, "", nil)
			svc, err := service.New(daemon, &service.Config{Name: "foo"})
			assert.NotError(t, err)
			assert.NotError(t, daemon.Start(svc))

			client, err := newRemoteClient(ctx, RPCService, "localhost", port, "")
			assert.NotError(t, err)

			return func() error { return daemon.Stop(svc) }, client
		},
		"RESTService": func(ctx context.Context, t *testing.T, _ jasper.Manager) (util.CloseFunc, remote.Manager) {
			port := testutil.GetPortNumber()
			manager := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

			daemon := newRESTDaemon("localhost", port, manager, nil)
			svc, err := service.New(daemon, &service.Config{Name: "foo"})
			assert.NotError(t, err)
			assert.NotError(t, daemon.Start(svc))
			assert.NotError(t, testutil.WaitForRESTService(ctx, fmt.Sprintf("http://localhost:%d/jasper/v1", port)))

			client, err := newRemoteClient(ctx, RESTService, "localhost", port, "")
			assert.NotError(t, err)

			return func() error { return daemon.Stop(svc) }, client
		},
		"CombinedServiceRESTClient": func(ctx context.Context, t *testing.T, manager jasper.Manager) (util.CloseFunc, remote.Manager) {
			restPort := testutil.GetPortNumber()
			daemon := newCombinedDaemon(
				newRESTDaemon("localhost", restPort, manager, nil),
				newRPCDaemon("localhost", testutil.GetPortNumber(), manager, "", nil),
			)
			svc, err := service.New(daemon, &service.Config{Name: "foo"})
			assert.NotError(t, err)
			assert.NotError(t, daemon.Start(svc))
			assert.NotError(t, testutil.WaitForRESTService(ctx, fmt.Sprintf("http://localhost:%d/jasper/v1", restPort)))

			client, err := newRemoteClient(ctx, RESTService, "localhost", restPort, "")
			assert.NotError(t, err)

			return func() error { return daemon.Stop(svc) }, client
		},
		"CombinedServiceRPCClient": func(ctx context.Context, t *testing.T, manager jasper.Manager) (util.CloseFunc, remote.Manager) {
			rpcPort := testutil.GetPortNumber()
			daemon := newCombinedDaemon(
				newRESTDaemon("localhost", testutil.GetPortNumber(), manager, nil),
				newRPCDaemon("localhost", rpcPort, manager, "", nil),
			)
			svc, err := service.New(daemon, &service.Config{Name: "foo"})
			assert.NotError(t, err)
			assert.NotError(t, daemon.Start(svc))

			client, err := newRemoteClient(ctx, RPCService, "localhost", rpcPort, "")
			assert.NotError(t, err)

			return func() error { return daemon.Stop(svc) }, client
		},
	} {
		t.Run(daemonAndClientName, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
			defer cancel()

			manager := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))
			closeDaemon, client := makeDaemonAndClient(ctx, t, manager)
			defer func() {
				check.NotError(t, closeDaemon())
			}()

			opts := &options.Create{
				Args: []string{"echo", "hello", "world"},
			}
			proc, err := client.CreateProcess(ctx, opts)
			assert.NotError(t, err)

			exitCode, err := proc.Wait(ctx)
			assert.NotError(t, err)
			assert.Zero(t, exitCode)
		})
	}
}
