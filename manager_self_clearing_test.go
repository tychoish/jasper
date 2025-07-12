package jasper

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func registerBasedCreate(ctx context.Context, m *selfClearingProcessManager, t *testing.T, _ *options.Create) (Process, error) {
	sleep, err := NewBlockingProcess(ctx, testutil.SleepCreateOpts(10))
	assert.NotError(t, err)
	assert.True(t, sleep != nil)
	err = m.Register(ctx, sleep)
	if err != nil {
		// Mimic the behavior of Create()'s error return.
		return nil, err
	}

	return sleep, err
}

func pureCreate(ctx context.Context, m *selfClearingProcessManager, _ *testing.T, opts *options.Create) (Process, error) {
	return m.CreateProcess(ctx, opts)
}

func fillUp(ctx context.Context, t *testing.T, manager *selfClearingProcessManager, numProcs int) {
	procs, err := createProcs(ctx, testutil.SleepCreateOpts(5), manager, numProcs)
	assert.NotError(t, err)
	assert.Equal(t, len(procs), numProcs)
}

func TestSelfClearingManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for mname, createFunc := range map[string]func(context.Context, *selfClearingProcessManager, *testing.T, *options.Create) (Process, error){
		"Create":   pureCreate,
		"Register": registerBasedCreate,
	} {
		t.Run(mname, func(t *testing.T) {

			for name, test := range map[string]func(context.Context, *testing.T, *selfClearingProcessManager, testutil.OptsModify){
				"SucceedsWhenFree": func(ctx context.Context, t *testing.T, manager *selfClearingProcessManager, mod testutil.OptsModify) {
					proc, err := createFunc(ctx, manager, t, testutil.TrueCreateOpts())
					assert.NotError(t, err)
					check.NotZero(t, proc.ID())
				},
				"ErrorsWhenFull": func(ctx context.Context, t *testing.T, manager *selfClearingProcessManager, mod testutil.OptsModify) {
					fillUp(ctx, t, manager, manager.maxProcs)

					sleep, err := createFunc(ctx, manager, t, testutil.SleepCreateOpts(10))
					check.Error(t, err)
					check.True(t, sleep == nil)
				},
				"PartiallySucceedsWhenAlmostFull": func(ctx context.Context, t *testing.T, manager *selfClearingProcessManager, mod testutil.OptsModify) {
					fillUp(ctx, t, manager, manager.maxProcs-1)
					firstSleep, err := createFunc(ctx, manager, t, testutil.SleepCreateOpts(10))
					assert.NotError(t, err)
					check.NotZero(t, firstSleep.ID())
					secondSleep, err := createFunc(ctx, manager, t, testutil.SleepCreateOpts(10))
					check.Error(t, err)
					check.True(t, secondSleep == nil)
				},
				"InitialFailureIsResolvedByWaiting": func(ctx context.Context, t *testing.T, manager *selfClearingProcessManager, mod testutil.OptsModify) {
					fillUp(ctx, t, manager, manager.maxProcs)
					sleepOpts := testutil.SleepCreateOpts(100)
					sleepProc, err := createFunc(ctx, manager, t, sleepOpts)
					check.Error(t, err)
					check.True(t, sleepProc == nil)
					otherSleepProcs, err := manager.List(ctx, options.All)
					assert.NotError(t, err)
					for _, otherSleepProc := range otherSleepProcs {
						_, err = otherSleepProc.Wait(ctx)
						assert.NotError(t, err)
					}
					sleepProc, err = createFunc(ctx, manager, t, sleepOpts)
					assert.NotError(t, err)
					check.True(t, sleepProc != nil)
				},
				//"": func(ctx context.Context, t *testing.T, manager *selfClearingProcessManager) {},
			} {

				t.Run("Blocking", func(t *testing.T) {
					t.Run(name, func(t *testing.T) {
						tctx, cancel := context.WithTimeout(ctx, testutil.ManagerTestTimeout)
						defer cancel()

						selfClearingManager := NewManager(ManagerOptionSet(ManagerOptions{
							MaxProcs: 5,
						}))

						test(tctx, t, selfClearingManager.(*selfClearingProcessManager), func(o *options.Create) {
							o.Implementation = options.ProcessImplementationBlocking
						})
						assert.NotError(t, selfClearingManager.Close(tctx))
					})
				})
				t.Run("Basic", func(t *testing.T) {
					t.Run(name, func(t *testing.T) {
						tctx, cancel := context.WithTimeout(ctx, testutil.ManagerTestTimeout)
						defer cancel()

						selfClearingManager := NewManager(ManagerOptionSet(ManagerOptions{
							MaxProcs: 5,
						}))

						test(tctx, t, selfClearingManager.(*selfClearingProcessManager), func(o *options.Create) {
							o.Implementation = options.ProcessImplementationBasic
						})
						assert.NotError(t, selfClearingManager.Close(tctx))
					})
				})

			}
		})
	}
}
