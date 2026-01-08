package testcases

import (
	"context"
	"runtime"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/fun/testt"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

var echoSubCmd = []string{"echo", "foo"}

type ManagerTest func(context.Context, *testing.T, jasper.Manager, testutil.OptsModify)

type ManagerSuite map[string]ManagerTest

func GenerateManagerSuite() ManagerSuite {
	return map[string]ManagerTest{
		"ValidateFixture": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			check.True(t, ctx != nil)
			check.True(t, manager != nil)
			check.True(t, manager.LoggingCache(ctx) != nil)
		},
		"IDReturnsNonempty": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			check.NotZero(t, manager.ID())
		},
		"ProcEnvVarMatchesManagerID": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)
			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			info := proc.Info(ctx)
			assert.True(t, info.Options.Environment.Len() != 0)
			lookup := stw.NewMap(map[string]string{})
			lookup.Extend(irt.KVsplit(info.Options.Environment.IteratorFront()))
			check.Equal(t, manager.ID(), lookup.Get(jasper.ManagerEnvironID))
		},
		"ListDoesNotErrorWhenEmpty": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			all, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, len(all), 0)
		},
		"CreateSimpleProcess": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)
			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			check.True(t, proc != nil)
			info := proc.Info(ctx)
			check.True(t, info.IsRunning || info.Complete)
		},
		"CreateProcessFails": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := &options.Create{}
			mod(opts)
			proc, err := manager.CreateProcess(ctx, opts)
			assert.Error(t, err)
			check.True(t, proc == nil)
		},
		"ListAllOperations": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)
			created, err := createProcs(ctx, opts, manager, 10)
			assert.NotError(t, err)
			check.Equal(t, len(created), 10)
			output, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, len(output), 10)
		},
		"ListAllReturnsErrorWithCanceledContext": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			cctx, cancel := context.WithCancel(ctx)
			opts := testutil.TrueCreateOpts()
			mod(opts)

			created, err := createProcs(ctx, opts, manager, 10)
			assert.NotError(t, err)
			check.Equal(t, len(created), 10)
			cancel()
			output, err := manager.List(cctx, options.All)
			assert.Error(t, err)
			check.True(t, output == nil)
		},
		"LongRunningOperationsAreListedAsRunning": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.SleepCreateOpts(10)
			mod(opts)

			procs, err := createProcs(ctx, opts, manager, 5)
			assert.NotError(t, err)
			check.Equal(t, len(procs), 5)

			procs, err = manager.List(ctx, options.Running)
			assert.NotError(t, err)
			check.Equal(t, len(procs), 5)

			procs, err = manager.List(ctx, options.Successful)
			assert.NotError(t, err)
			check.Equal(t, len(procs), 0)
		},
		"ListReturnsOneSuccessfulCommand": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)

			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)

			_, err = proc.Wait(ctx)
			assert.NotError(t, err)

			listOut, err := manager.List(ctx, options.Successful)
			assert.NotError(t, err)

			assert.Equal(t, len(listOut), 1)
			check.Equal(t, listOut[0].ID(), proc.ID())
		},
		"ListReturnsOneFailedCommand": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.FalseCreateOpts()
			mod(opts)

			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			_, err = proc.Wait(ctx)
			assert.Error(t, err)

			listOut, err := manager.List(ctx, options.Failed)
			assert.NotError(t, err)

			assert.Equal(t, len(listOut), 1)
			check.Equal(t, listOut[0].ID(), proc.ID())
		},
		"GetMethodErrorsWithNoResponse": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			proc, err := manager.Get(ctx, "foo")
			assert.Error(t, err)
			check.True(t, proc == nil)
		},
		"GetMethodReturnsMatchingDoc": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)
			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)

			ret, err := manager.Get(ctx, proc.ID())
			assert.NotError(t, err)
			check.Equal(t, ret.ID(), proc.ID())
		},
		"GroupDoesNotErrorWithoutResults": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			procs, err := manager.Group(ctx, "foo")
			assert.NotError(t, err)
			check.Equal(t, len(procs), 0)
		},
		"GroupErrorsForCanceledContexts": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)

			_, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)

			cctx, cancel := context.WithCancel(ctx)
			cancel()
			procs, err := manager.Group(cctx, "foo")
			assert.Error(t, err)
			check.Equal(t, len(procs), 0)
			check.Substring(t, err.Error(), context.Canceled.Error())
		},
		"GroupPropagatesMatching": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)

			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)

			proc.Tag("foo")

			procs, err := manager.Group(ctx, "foo")
			assert.NotError(t, err)
			assert.Equal(t, len(procs), 1)
			check.Equal(t, procs[0].ID(), proc.ID())
		},
		"CloseEmptyManagerNoops": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			check.NotError(t, manager.Close(ctx))
		},
		"CloseErrorsWithCanceledContext": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.SleepCreateOpts(100)
			mod(opts)

			_, err := createProcs(ctx, opts, manager, 10)
			assert.NotError(t, err)

			cctx, cancel := context.WithCancel(ctx)
			cancel()

			err = manager.Close(cctx)
			assert.Error(t, err)
			check.Substring(t, err.Error(), context.Canceled.Error())
		},
		"CloseSucceedsWithTerminatedProcesses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)

			procs, err := createProcs(ctx, opts, manager, 10)
			for _, p := range procs {
				_, err = p.Wait(ctx)
				assert.NotError(t, err)
			}

			assert.NotError(t, err)
			check.NotError(t, manager.Close(ctx))
		},
		"ClosersWithoutTriggersTerminatesProcesses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			if runtime.GOOS == "windows" {
				t.Skip("manager close tests will error due to process termination on Windows")
			}
			opts := testutil.SleepCreateOpts(100)
			mod(opts)

			_, err := createProcs(ctx, opts, manager, 10)
			assert.NotError(t, err)
			check.NotError(t, manager.Close(ctx))
		},
		"CloseExecutesClosersForProcesses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			if runtime.GOOS == "windows" {
				t.Skip("manager close tests will error due to process termination on Windows")
			}
			opts := testutil.SleepCreateOpts(5)
			mod(opts)

			count := 0
			countIncremented := make(chan bool, 1)
			opts.RegisterCloser(func() (_ error) {
				count++
				countIncremented <- true
				close(countIncremented)
				return
			})

			_, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)

			check.Equal(t, count, 0)
			assert.NotError(t, manager.Close(ctx))
			select {
			case <-ctx.Done():
				t.Fatal("process took too long to run closers")
			case <-countIncremented:
				check.Equal(t, 1, count)
			}
		},
		"RegisterProcessErrorsForNilProcess": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			err := manager.Register(ctx, nil)
			assert.Error(t, err)
			check.Substring(t, err.Error(), "not defined")
		},
		// "RegisterProcessErrorsForCanceledContext": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
		// 	cctx, cancel := context.WithCancel(ctx)
		// 	cancel()

		// 	opts := testutil.TrueCreateOpts()
		// 	mod(opts)

		// 	proc, err := NewBlockingProcess(ctx, opts)
		// 	assert.NotError(t, err)
		// 	err = manager.Register(cctx, proc)
		// 	assert.Error(t, err)
		// 	check.Contains(t, err.Error(), context.Canceled.Error())
		// },
		// "RegisterProcessErrorsWhenMissingID": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
		// 	proc := &blockingProcess{}
		// 	check.Equal(t, proc.ID(), "")
		// 	err := manager.Register(ctx, proc)
		// 	assert.Error(t, err)
		// 	check.Contains(t, err.Error(), "malformed")
		// },
		// "RegisterProcessModifiesManagerState": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
		// 	opts := testutil.TrueCreateOpts()
		// 	mod(opts)

		// 	proc, err := newBlockingProcess(ctx, opts)
		// 	assert.NotError(t, err)
		// 	err = manager.Register(ctx, proc)
		// 	assert.NotError(t, err)

		// 	procs, err := manager.List(ctx, options.All)
		// 	assert.NotError(t, err)
		// 	assert.True(t, len(procs) >= 1)

		// 	check.Equal(t, procs[0].ID(), proc.ID())
		// },
		// "RegisterProcessErrorsForDuplicateProcess": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
		// 	opts := testutil.TrueCreateOpts()
		// 	mod(opts)

		// 	proc, err := newBlockingProcess(ctx, opts)
		// 	assert.NotError(t, err)
		// 	check.NotEmpty(t, proc)
		// 	err = manager.Register(ctx, proc)
		// 	assert.NotError(t, err)
		// 	err = manager.Register(ctx, proc)
		// 	check.Error(t, err)
		// },
		"ManagerCallsOptionsCloseByDefault": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := &options.Create{}
			mod(opts)
			opts.Args = []string{"echo", "foobar"}
			count := 0
			countIncremented := make(chan bool, 1)
			opts.RegisterCloser(func() (_ error) {
				count++
				countIncremented <- true
				close(countIncremented)
				return
			})

			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			_, err = proc.Wait(ctx)
			assert.NotError(t, err)

			select {
			case <-ctx.Done():
				t.Fatal("process took too long to run closers")
			case <-countIncremented:
				check.Equal(t, 1, count)
			}
		},
		"ClearCausesDeletionOfProcesses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.TrueCreateOpts()
			mod(opts)
			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			sameProc, err := manager.Get(ctx, proc.ID())
			assert.NotError(t, err)
			assert.Equal(t, proc.ID(), sameProc.ID())
			_, err = proc.Wait(ctx)
			assert.NotError(t, err)
			manager.Clear(ctx)
			nilProc, err := manager.Get(ctx, proc.ID())
			assert.Error(t, err)
			check.True(t, nilProc == nil)
		},
		"ClearIsANoopForActiveProcesses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			opts := testutil.SleepCreateOpts(20)
			mod(opts)
			proc, err := manager.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			manager.Clear(ctx)
			sameProc, err := manager.Get(ctx, proc.ID())
			assert.NotError(t, err)
			check.Equal(t, proc.ID(), sameProc.ID())
			assert.NotError(t, jasper.Terminate(ctx, proc)) // Clean up
		},
		"ClearSelectivelyDeletesOnlyDeadProcesses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			trueOpts := testutil.TrueCreateOpts()
			mod(trueOpts)
			lsProc, err := manager.CreateProcess(ctx, trueOpts)
			assert.NotError(t, err)

			sleepOpts := testutil.SleepCreateOpts(20)
			mod(sleepOpts)
			sleepProc, err := manager.CreateProcess(ctx, sleepOpts)
			assert.NotError(t, err)

			_, err = lsProc.Wait(ctx)
			assert.NotError(t, err)

			manager.Clear(ctx)

			sameSleepProc, err := manager.Get(ctx, sleepProc.ID())
			assert.NotError(t, err)
			check.Equal(t, sleepProc.ID(), sameSleepProc.ID())

			nilProc, err := manager.Get(ctx, lsProc.ID())
			assert.Error(t, err)
			check.True(t, nilProc == nil)
			assert.NotError(t, jasper.Terminate(ctx, sleepProc)) // Clean up
		},
		"CreateCommandPasses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			cmd := manager.CreateCommand(ctx)
			cmd.Add(echoSubCmd)
			cmd.WithOptions(func(opts *options.Command) {
				mod(&opts.Process)
			})
			check.NotError(t, cmd.Run(ctx))
		},
		"RunningCommandCreatesNewProcesses": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			procList, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			originalProcCount := len(procList) // zero
			cmd := manager.CreateCommand(ctx)
			subCmds := [][]string{echoSubCmd, echoSubCmd, echoSubCmd}
			cmd.Extend(subCmds)
			cmd.WithOptions(func(opts *options.Command) {
				mod(&opts.Process)
			})
			assert.NotError(t, cmd.Run(ctx))
			newProcList, err := manager.List(ctx, options.All)
			assert.NotError(t, err)

			check.Equal(t, len(newProcList), originalProcCount+len(subCmds))
		},
		"CommandProcessIDsMatchManagedProcessIDs": func(ctx context.Context, t *testing.T, manager jasper.Manager, mod testutil.OptsModify) {
			cmd := manager.CreateCommand(ctx)
			cmd.Extend([][]string{echoSubCmd, echoSubCmd, echoSubCmd})
			cmd.WithOptions(func(opts *options.Command) {
				mod(&opts.Process)
			})
			assert.NotError(t, cmd.Run(ctx))
			newProcList, err := manager.List(ctx, options.All)
			assert.NotError(t, err)

			findIDInProcList := func(procID string) bool {
				for _, proc := range newProcList {
					if proc.ID() == procID {
						return true
					}
				}
				return false
			}

			for _, procID := range cmd.GetProcIDs() {
				check.True(t, findIDInProcList(procID))
			}
		},
	}
}

func RunManagerSuite(t *testing.T, suite ManagerSuite, makeMngr func(context.Context, *testing.T) jasper.Manager) {
	ctx := testt.Context(t)
	for name, test := range suite {
		t.Run(name+"/BasicProcess", func(t *testing.T) {
			tctx, tcancel := context.WithTimeout(ctx, testutil.ManagerTestTimeout)
			defer tcancel()
			test(tctx, t, makeMngr(tctx, t), func(opts *options.Create) {
				opts.Implementation = options.ProcessImplementationBlocking
			})
		})
		t.Run(name+"/BlockingProcess", func(t *testing.T) {
			tctx, tcancel := context.WithTimeout(ctx, testutil.ManagerTestTimeout)
			defer tcancel()
			test(tctx, t, makeMngr(tctx, t), func(opts *options.Create) {
				opts.Implementation = options.ProcessImplementationBlocking
			})
		})
	}
}
