package remote

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

type ProcessTestCase struct {
	Name       string
	Case       func(context.Context, *testing.T, *options.Create, jasper.ProcessConstructor)
	ShouldSkip bool
}

func AddBasicProcessTests(tests ...ProcessTestCase) []ProcessTestCase {
	return append([]ProcessTestCase{
		{
			Name: "WithPopulatedArgsCommandCreationPasses",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				assert.True(t, opts.Args != nil)
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)
			},
		},
		{
			Name: "ErrorToCreateWithInvalidArgs",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				opts.Args = []string{}
				proc, err := makep(ctx, opts)
				assert.Error(t, err)
				assert.True(t, proc == nil)
			},
		},
		{
			Name: "WithCanceledContextProcessCreationFails",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				pctx, pcancel := context.WithCancel(ctx)
				pcancel()
				proc, err := makep(pctx, opts)
				assert.Error(t, err)
				assert.True(t, proc == nil)
			},
		},
		{
			Name: "CanceledContextTimesOutEarly",
			Case: func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
				pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
				defer pcancel()
				startAt := time.Now()
				opts := testutil.SleepCreateOpts(20)
				proc, err := makep(pctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)

				time.Sleep(5 * time.Millisecond) // let time pass...
				assert.True(t, !proc.Info(ctx).Successful)
				check.True(t, time.Since(startAt) < 20*time.Second)
			},
		},
		{
			Name: "ProcessLacksTagsByDefault",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				tags := proc.GetTags()
				assert.Equal(t, len(tags), 0)
			},
		},
		{
			Name: "ProcessTagsPersist",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				opts.Tags = []string{"foo"}
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				tags := proc.GetTags()
				assert.Contains(t, tags, "foo")
			},
		},
		{
			Name: "InfoHasMatchingID",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				_, err = proc.Wait(ctx)
				assert.NotError(t, err)
				assert.Equal(t, proc.ID(), proc.Info(ctx).ID)
			},
		},
		{
			Name: "ResetTags",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				proc.Tag("foo")
				assert.Contains(t, proc.GetTags(), "foo")
				proc.ResetTags()
				assert.Equal(t, len(proc.GetTags()), 0)
			},
		},
		{
			Name: "TagsAreSetLike",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)

				for i := 0; i < 50; i++ {
					proc.Tag("foo")
				}

				assert.Equal(t, len(proc.GetTags()), 1)
				proc.Tag("bar")
				assert.Equal(t, len(proc.GetTags()), 2)
			},
		},
		{
			Name: "CompleteIsTrueAfterWait",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				time.Sleep(10 * time.Millisecond) // give the process time to start background machinery
				_, err = proc.Wait(ctx)
				check.NotError(t, err)
				check.True(t, proc.Complete(ctx))
			},
		},
		{
			Name: "WaitReturnsWithCanceledContext",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				opts.Args = []string{"sleep", "10"}
				pctx, pcancel := context.WithCancel(ctx)
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				check.True(t, proc.Running(ctx))
				check.NotError(t, err)
				pcancel()
				_, err = proc.Wait(pctx)
				assert.Error(t, err)
			},
		},
		{
			Name: "RegisterTriggerErrorsForNil",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.Error(t, proc.RegisterTrigger(ctx, nil))
			},
		},
		{
			Name: "RegisterSignalTriggerIDErrorsForExitedProcess",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				_, err = proc.Wait(ctx)
				check.NotError(t, err)
				assert.Error(t, proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger))
			},
		},
		{
			Name: "RegisterSignalTriggerIDFailsWithInvalidTriggerID",
			Case: func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
				opts := testutil.SleepCreateOpts(3)
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.Error(t, proc.RegisterSignalTriggerID(ctx, jasper.SignalTriggerID(fmt.Sprintln(-1))))
			},
		},
		{
			Name: "RegisterSignalTriggerIDPassesWithValidTriggerID",
			Case: func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
				opts := testutil.SleepCreateOpts(3)
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				check.NotError(t, proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger))
			},
		},
		{
			Name: "WaitOnRespawnedProcessDoesNotError",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)
				_, err = proc.Wait(ctx)
				assert.NotError(t, err)

				newProc, err := proc.Respawn(ctx)
				assert.NotError(t, err)
				_, err = newProc.Wait(ctx)
				check.NotError(t, err)
			},
		},
		{
			Name: "RespawnedProcessGivesSameResult",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)

				_, err = proc.Wait(ctx)
				assert.NotError(t, err)
				procExitCode := proc.Info(ctx).ExitCode

				newProc, err := proc.Respawn(ctx)
				assert.NotError(t, err)
				_, err = newProc.Wait(ctx)
				assert.NotError(t, err)
				assert.Equal(t, procExitCode, newProc.Info(ctx).ExitCode)
			},
		},
		{
			Name: "RespawningFinishedProcessIsOK",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)
				_, err = proc.Wait(ctx)
				assert.NotError(t, err)

				newProc, err := proc.Respawn(ctx)
				check.NotError(t, err)
				_, err = newProc.Wait(ctx)
				assert.NotError(t, err)
				check.True(t, newProc.Info(ctx).Successful)
			},
		},
		{
			Name: "RespawningRunningProcessIsOK",
			Case: func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
				opts := testutil.SleepCreateOpts(2)
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)

				newProc, err := proc.Respawn(ctx)
				check.NotError(t, err)
				_, err = newProc.Wait(ctx)
				assert.NotError(t, err)
				check.True(t, newProc.Info(ctx).Successful)
			},
		},
		{
			Name: "RespawnShowsConsistentStateValues",
			Case: func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
				opts := testutil.SleepCreateOpts(3)
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)
				_, err = proc.Wait(ctx)
				assert.NotError(t, err)

				newProc, err := proc.Respawn(ctx)
				assert.NotError(t, err)
				check.True(t, newProc.Running(ctx))
				_, err = newProc.Wait(ctx)
				assert.NotError(t, err)
				check.True(t, proc.Complete(ctx))
			},
		},
		{
			Name: "WaitGivesSuccessfulExitCode",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, testutil.TrueCreateOpts())
				assert.NotError(t, err)
				assert.True(t, proc != nil)
				exitCode, err := proc.Wait(ctx)
				check.NotError(t, err)
				assert.Equal(t, 0, exitCode)
			},
		},
		{
			Name: "WaitGivesFailureExitCode",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, testutil.FalseCreateOpts())
				assert.NotError(t, err)
				assert.True(t, proc != nil)
				exitCode, err := proc.Wait(ctx)
				assert.Error(t, err)
				assert.Equal(t, 1, exitCode)
			},
		},
		{
			Name: "WaitGivesProperExitCodeOnSignalDeath",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, testutil.SleepCreateOpts(100))
				assert.NotError(t, err)
				assert.True(t, proc != nil)
				sig := syscall.SIGTERM
				assert.NotError(t, proc.Signal(ctx, sig))
				exitCode, err := proc.Wait(ctx)
				assert.Error(t, err)
				if runtime.GOOS == "windows" {
					assert.Equal(t, 1, exitCode)
				} else {
					assert.Equal(t, int(sig), exitCode)
				}
			},
		},
		{
			Name: "WaitGivesNegativeOneOnAlternativeError",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, testutil.SleepCreateOpts(100))
				assert.NotError(t, err)
				assert.True(t, proc != nil)

				var exitCode int
				waitFinished := make(chan bool)
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				go func() {
					exitCode, err = proc.Wait(cctx)
					waitFinished <- true
				}()
				select {
				case <-waitFinished:
					assert.Error(t, err)
					assert.Equal(t, -1, exitCode)
				case <-ctx.Done():
					t.Error("call to Wait() took too long to finish")
				}
			},
		},
		{
			Name: "InfoHasTimeoutWhenProcessTimesOut",
			Case: func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
				opts := testutil.SleepCreateOpts(100)
				opts.Timeout = time.Second
				opts.TimeoutSecs = 1
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)

				exitCode, err := proc.Wait(ctx)
				assert.Error(t, err)
				if runtime.GOOS == "windows" {
					assert.Equal(t, 1, exitCode)
				} else {
					assert.Equal(t, int(syscall.SIGKILL), exitCode)
				}
				info := proc.Info(ctx)
				check.True(t, info.Timeout)
			},
		},
		{
			Name: "CallingSignalOnDeadProcessDoesError",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)

				_, err = proc.Wait(ctx)
				check.NotError(t, err)

				err = proc.Signal(ctx, syscall.SIGTERM)
				assert.Error(t, err)
				check.True(t, strings.Contains(err.Error(), "cannot signal a process that has terminated"))
			},
		},
		{
			Name: "CompleteAlwaysReturnsTrueWhenProcessIsComplete",
			Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)

				_, err = proc.Wait(ctx)
				check.NotError(t, err)

				check.True(t, proc.Complete(ctx))
			},
		},
		{
			Name: "RegisterSignalTriggerFails",
			Case: func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
				opts := testutil.SleepCreateOpts(3)
				proc, err := makep(ctx, opts)
				assert.NotError(t, err)
				assert.Error(t, proc.RegisterSignalTrigger(ctx, func(_ jasper.ProcessInfo, _ syscall.Signal) bool {
					return false
				}))
			},
		},
	}, tests...)

}

func TestProcessImplementations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpClient := testutil.GetHTTPClient()
	defer testutil.PutHTTPClient(httpClient)

	for cname, makeProc := range map[string]jasper.ProcessConstructor{
		"REST": func(ctx context.Context, opts *options.Create) (jasper.Process, error) {
			_, port, err := startRESTService(ctx, httpClient)
			if err != nil {
				return nil, err
			}

			client := &restClient{
				prefix: fmt.Sprintf("http://localhost:%d/jasper/v1", port),
				client: httpClient,
			}

			return client.CreateProcess(ctx, opts)
		},
		"MDB": func(ctx context.Context, opts *options.Create) (jasper.Process, error) {
			mngr := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

			client, err := makeTestMDBServiceAndClient(ctx, mngr)
			if err != nil {
				return nil, err
			}

			return client.CreateProcess(ctx, opts)
		},
		"RPC/TLS": func(ctx context.Context, opts *options.Create) (jasper.Process, error) {
			mngr := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

			client, err := makeTLSRPCServiceAndClient(ctx, mngr)
			if err != nil {
				return nil, err
			}

			return client.CreateProcess(ctx, opts)
		},
		"RPC/Insecure": func(ctx context.Context, opts *options.Create) (jasper.Process, error) {
			mngr := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

			client, err := makeInsecureRPCServiceAndClient(ctx, mngr)
			if err != nil {
				return nil, err
			}

			return client.CreateProcess(ctx, opts)
		},
	} {
		if cname == "MDB" {
			t.SkipNow()
		}
		t.Run(cname, func(t *testing.T) {
			for _, modify := range []struct {
				Name    string
				Options testutil.OptsModify
			}{
				{
					Name: "Blocking",
					Options: func(opts *options.Create) {
						opts.Implementation = options.ProcessImplementationBlocking
					},
				},
				{
					Name: "Basic",
					Options: func(opts *options.Create) {
						opts.Implementation = options.ProcessImplementationBasic
					},
				},
				{
					Name:    "Default",
					Options: func(opts *options.Create) {},
				},
			} {
				t.Run(modify.Name, func(t *testing.T) {
					for _, test := range AddBasicProcessTests(
						ProcessTestCase{
							Name:       "CompleteReturnsFalseForProcessThatDoesntExist",
							ShouldSkip: !strings.Contains(cname, "RPC"),
							Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
								proc, err := makep(ctx, opts)
								assert.NotError(t, err)

								firstID := proc.ID()
								_, err = proc.Wait(ctx)
								check.NotError(t, err)
								check.True(t, proc.Complete(ctx))
								proc.(*rpcProcess).info.Id += "_foo"
								proc.(*rpcProcess).info.Complete = false
								assert.NotEqual(t, firstID, proc.ID())
								assert.True(t, !proc.Complete(ctx))
							},
						},
						ProcessTestCase{
							Name:       "RunningReturnsFalseForProcessThatDoesntExist",
							ShouldSkip: !strings.Contains(cname, "RPC"),
							Case: func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
								proc, err := makep(ctx, opts)
								assert.NotError(t, err)

								firstID := proc.ID()
								_, err = proc.Wait(ctx)
								check.NotError(t, err)
								proc.(*rpcProcess).info.Id += "_foo"
								proc.(*rpcProcess).info.Complete = false
								assert.NotEqual(t, firstID, proc.ID())
								assert.True(t, !proc.Running(ctx))
							},
						},
					) {
						if test.ShouldSkip {
							continue
						}
						t.Run(test.Name, func(t *testing.T) {
							tctx, cancel := context.WithTimeout(ctx, testutil.RPCTestTimeout)
							defer cancel()

							opts := &options.Create{Args: []string{"ls"}}
							modify.Options(opts)
							test.Case(tctx, t, opts, makeProc)
						})
					}
				})
			}
		})
	}
}
