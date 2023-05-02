package jasper

import (
	"context"
	"errors"

	"github.com/tychoish/jasper/options"
)

type selfClearingProcessManager struct {
	*basicProcessManager
	maxProcs int
}

func (m *selfClearingProcessManager) checkProcCapacity(ctx context.Context) error {
	if len(m.basicProcessManager.procs) == m.maxProcs {
		// We are at capacity, we can try to perform a clear.
		m.Clear(ctx)
		if len(m.basicProcessManager.procs) == m.maxProcs {
			return errors.New("cannot create any more processes")
		}
	}

	return nil
}

func (m *selfClearingProcessManager) CreateProcess(ctx context.Context, opts *options.Create) (Process, error) {
	if err := m.checkProcCapacity(ctx); err != nil {
		return nil, err
	}

	proc, err := m.basicProcessManager.CreateProcess(ctx, opts)
	if err != nil {
		return nil, err
	}

	return proc, nil
}

func (m *selfClearingProcessManager) Register(ctx context.Context, proc Process) error {
	if err := m.checkProcCapacity(ctx); err != nil {
		return err
	}

	return m.basicProcessManager.Register(ctx, proc)
}
