package jasper

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func TestTrackedManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for managerName, makeManager := range map[string]func() *basicProcessManager{
		"Basic": func() *basicProcessManager {
			return &basicProcessManager{
				procs:   map[string]Process{},
				loggers: NewLoggingCache(),
				tracker: &mockProcessTracker{
					Infos: []ProcessInfo{},
				},
			}
		},
	} {
		t.Run(managerName, func(t *testing.T) {
			for name, test := range map[string]func(context.Context, *testing.T, *basicProcessManager, *options.Create){
				"ValidateFixtureSetup": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					check.True(t, nil != manager.tracker)
					check.Equal(t, len(manager.procs), 0)
					check.True(t, nil != manager.LoggingCache(ctx))
				},
				"CreateProcessTracksProcess": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					proc, err := manager.CreateProcess(ctx, opts)
					assert.NotError(t, err)
					check.Equal(t, len(manager.procs), 1)

					mockTracker, ok := manager.tracker.(*mockProcessTracker)
					assert.True(t, ok)
					assert.Equal(t, len(mockTracker.Infos), 1)
					check.Equal(t, proc.Info(ctx).ID, mockTracker.Infos[0].ID)
				},
				"CreateCommandTracksCommandAfterRun": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					err := manager.CreateCommand(ctx).Add(opts.Args).Background(true).Run(ctx)
					assert.NotError(t, err)
					check.Equal(t, len(manager.procs), 1)

					mockTracker, ok := manager.tracker.(*mockProcessTracker)
					assert.True(t, ok)
					assert.Equal(t, len(mockTracker.Infos), 1)
					assert.True(t, len(mockTracker.Infos[0].Options.Args) != 0)
				},
				"DoNotTrackProcessIfCreateProcessDoesNotMakeProcess": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					opts.Args = []string{"foo"}
					_, err := manager.CreateProcess(ctx, opts)
					assert.Error(t, err)
					check.Equal(t, len(manager.procs), 0)

					mockTracker, ok := manager.tracker.(*mockProcessTracker)
					assert.True(t, ok)
					check.Equal(t, len(mockTracker.Infos), 0)
				},
				"DoNotTrackProcessIfCreateCommandDoesNotMakeProcess": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					opts.Args = []string{"foo"}
					cmd := manager.CreateCommand(ctx).Add(opts.Args).Background(true)
					cmd.opts.Process = *opts
					err := cmd.Run(ctx)
					assert.Error(t, err)
					check.Equal(t, len(manager.procs), 0)

					mockTracker, ok := manager.tracker.(*mockProcessTracker)
					assert.True(t, ok)
					check.Equal(t, len(mockTracker.Infos), 0)
				},
				"CloseCleansUpProcesses": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					cmd := manager.CreateCommand(ctx).Background(true).Add(opts.Args)
					cmd.opts.Process = *opts
					assert.NotError(t, cmd.Run(ctx))
					check.Equal(t, len(manager.procs), 1)

					mockTracker, ok := manager.tracker.(*mockProcessTracker)
					assert.True(t, ok)
					assert.Equal(t, len(mockTracker.Infos), 1)
					check.NotZero(t, mockTracker.Infos[0].ID)

					assert.NotError(t, manager.Close(ctx))
					check.Equal(t, len(mockTracker.Infos), 0)
					assert.NotError(t, manager.Close(ctx))
				},
				"CloseWithNoProcessesIsNotError": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					mockTracker, ok := manager.tracker.(*mockProcessTracker)
					assert.True(t, ok)

					assert.NotError(t, manager.Close(ctx))
					check.Equal(t, len(mockTracker.Infos), 0)
					assert.NotError(t, manager.Close(ctx))
					check.Equal(t, len(mockTracker.Infos), 0)
				},
				"DoubleCloseIsNotError": func(ctx context.Context, t *testing.T, manager *basicProcessManager, opts *options.Create) {
					cmd := manager.CreateCommand(ctx).Background(true).Add(opts.Args)
					cmd.opts.Process = *opts

					assert.NotError(t, cmd.Run(ctx))
					check.Equal(t, len(manager.procs), 1)

					mockTracker, ok := manager.tracker.(*mockProcessTracker)
					assert.True(t, ok)
					assert.Equal(t, len(mockTracker.Infos), 1)
					check.NotZero(t, mockTracker.Infos[0].ID)

					assert.NotError(t, manager.Close(ctx))
					check.Equal(t, len(mockTracker.Infos), 0)
					assert.NotError(t, manager.Close(ctx))
					check.Equal(t, len(mockTracker.Infos), 0)
				},
				// "": func(ctx context.Context, t *testing.T, manager Manager, mod testutil.OptsModify) {},
			} {
				tctx, cancel := context.WithTimeout(ctx, testutil.ManagerTestTimeout)
				defer cancel()
				t.Run(name+"Manager/BlockingProcess", func(t *testing.T) {
					opts := testutil.SleepCreateOpts(1)
					opts.Implementation = options.ProcessImplementationBlocking
					test(tctx, t, makeManager(), opts)
				})
				t.Run(name+"Manager/BasicProcess", func(t *testing.T) {
					opts := testutil.SleepCreateOpts(1)
					opts.Implementation = options.ProcessImplementationBasic
					test(tctx, t, makeManager(), opts)
				})
			}
		})
	}
}
