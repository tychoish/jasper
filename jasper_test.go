package jasper

import (
	"context"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper/options"
)

func makeLockingProcess(pmake ProcessConstructor) ProcessConstructor {
	return func(ctx context.Context, opts *options.Create) (Process, error) {
		proc, err := pmake(ctx, opts)
		if err != nil {
			return nil, err
		}

		return SyncrhonizeProcess(proc), nil
	}
}

func createProcs(ctx context.Context, opts *options.Create, manager Manager, num int) ([]Process, error) {
	catcher := &erc.Collector{}
	out := []Process{}
	for i := 0; i < num; i++ {
		optsCopy := *opts

		proc, err := manager.CreateProcess(ctx, &optsCopy)
		catcher.Push(err)
		if proc != nil {
			out = append(out, proc)
		}
	}

	return out, catcher.Resolve()
}
