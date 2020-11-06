package jasper

import (
	"context"

	"github.com/deciduosity/jasper/options"
)

type wsepManager struct {
	Manager
	opts *options.WebSocketExec
}

// NewWebSocketExecManager returns a manager in which each process is
// created using a websocket execution protocol (wsep).
func NewWebSocketExecManager(m Manager, opts *options.WebSocketExec) Manager {
	return &wsepManager{
		Manager: m,
		opts:    opts,
	}
}

func (m *wsepManager) CreateProcess(ctx context.Context, opts *options.Create) (Process, error) {
	opts.WebSocketExec = m.opts
	return m.Manager.CreateProcess(ctx, opts)
}

func (m *wsepManager) CreateCommand(ctx context.Context) *Command {
	cmd := m.Manager.CreateCommand(ctx)
	cmd.opts.Process.WebSocketExec = m.opts
	return cmd
}
