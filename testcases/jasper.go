package testcases

import (
	"context"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

func makeLockingProcess(pmake jasper.ProcessConstructor) jasper.ProcessConstructor {
	return func(ctx context.Context, opts *options.Create) (jasper.Process, error) {
		proc, err := pmake(ctx, opts)
		if err != nil {
			return nil, err
		}

		return jasper.SyncrhonizeProcess(proc), nil
	}
}

func createProcs(ctx context.Context, opts *options.Create, manager jasper.Manager, num int) ([]jasper.Process, error) {
	catcher := &erc.Collector{}
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
