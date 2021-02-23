package remote

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testutil"
)

func makeTestMDBServiceAndClient(ctx context.Context, mngr jasper.Manager) (Manager, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", testutil.GetPortNumber()))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	closeService, err := StartMDBService(ctx, mngr, addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	go func() {
		<-ctx.Done()
		grip.Notice(closeService())
	}()
	if err = testutil.WaitForWireService(ctx, addr); err != nil {
		return nil, errors.WithStack(err)
	}

	client, err := NewMDBClient(ctx, addr, 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	go func() {
		<-ctx.Done()
		grip.Notice(client.CloseConnection())
	}()
	return client, nil
}
