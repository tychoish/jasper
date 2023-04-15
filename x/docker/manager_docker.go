package docker

import (
	"context"

	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

type dockerManager struct {
	jasper.Manager
	opts *options.Docker
}

// NewDockerManager returns a manager in which each process is created within
// a Docker container with the given options.
func NewDockerManager(m jasper.Manager, opts *options.Docker) jasper.Manager {
	return &dockerManager{
		Manager: m,
		opts:    opts,
	}
}

func (m *dockerManager) CreateProcess(ctx context.Context, opts *options.Create) (jasper.Process, error) {
	opts.Docker = m.opts
	return m.Manager.CreateProcess(ctx, opts)
}

func (m *dockerManager) CreateCommand(ctx context.Context) *jasper.Command {
	cmd := m.Manager.CreateCommand(ctx)
	cmd.WithOptions(func(opts *options.Command) {
		opts.Process.Docker = m.opts

	})
	return cmd
}
