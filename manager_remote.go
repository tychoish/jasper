package jasper

import (
	"context"

	"github.com/tychoish/jasper/options"
)

type remoteOverrideMgr struct {
	Manager
	remote *options.Remote
}

func (m *remoteOverrideMgr) CreateProcess(ctx context.Context, opts *options.Create) (Process, error) {
	opts.Remote = m.remote
	return m.Manager.CreateProcess(ctx, opts)
}

func (m *remoteOverrideMgr) CreateCommand(ctx context.Context) *Command {
	cmd := m.Manager.CreateCommand(ctx)
	cmd.opts.Process.Remote = m.remote
	return cmd
}
