package jasper

import (
	"context"
	"fmt"

	"github.com/tychoish/jasper/options"
)

// NewProcess is a factory function which constructs a local Process outside
// of the context of a manager.
func NewProcess(ctx context.Context, opts *options.Create) (Process, error) {
	var (
		proc Process
		err  error
	)

	if err = opts.Validate(); err != nil {
		return nil, (err)
	}

	switch opts.Implementation {
	case options.ProcessImplementationBlocking:
		proc, err = NewBlockingProcess(ctx, opts)
		if err != nil {
			return nil, err
		}
	case options.ProcessImplementationBasic:
		proc, err = NewBasicProcess(ctx, opts)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("cannot create '%s' type of process", opts.Implementation)
	}

	if !opts.Synchronized {
		return proc, nil
	}
	return &synchronizedProcess{proc: proc}, nil
}
