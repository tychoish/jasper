package jasper

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper/executor"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

const gracefulTimeout = 1000 * time.Millisecond

func TestBlockingProcess(t *testing.T) {
	t.Parallel()
	// we run the suite multiple times given that implementation
	// is heavily threaded, there are timing concerns that require
	// multiple executions.
	for _, attempt := range []string{"First", "Second", "Third", "Fourth", "Fifth"} {
		t.Run(attempt, func(t *testing.T) {
			t.Parallel()
			for name, testCase := range map[string]func(context.Context, *testing.T, *blockingProcess){
				"VerifyTestCaseConfiguration": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					check.NotZero(t, proc)
					check.True(t, ctx != nil)
					check.NotZero(t, proc.ID())
					check.True(t, !proc.Complete(ctx))
					check.True(t, nil != MakeDefaultTrigger(ctx, nil, &proc.info.Options, "foo"))
				},
				"InfoIDPopulatedInBasicCase": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					infoReturned := make(chan struct{})
					go func() {
						check.Equal(t, proc.Info(ctx).ID, proc.ID())
						close(infoReturned)
					}()

					op := <-proc.ops
					op(nil)
					<-infoReturned
				},
				"InfoReturnsNotCompleteForCanceledCase": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					signal := make(chan struct{})
					go func() {
						cctx, cancel := context.WithCancel(ctx)
						cancel()

						check.True(t, !proc.Info(cctx).Complete)
						close(signal)
					}()

					gracefulCtx, cancel := context.WithTimeout(ctx, gracefulTimeout)
					defer cancel()

					select {
					case <-signal:
					case <-gracefulCtx.Done():
						t.Error("reached timeout")
					}
				},
				"SignalErrorsForCanceledContext": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					signal := make(chan struct{})
					go func() {
						cctx, cancel := context.WithCancel(ctx)
						cancel()

						check.Error(t, proc.Signal(cctx, syscall.SIGTERM))
						close(signal)
					}()

					gracefulCtx, cancel := context.WithTimeout(ctx, gracefulTimeout)
					defer cancel()

					select {
					case <-signal:
					case <-gracefulCtx.Done():
						t.Error("reached timeout")
					}
				},
				"TestRegisterTriggerAfterComplete": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.info.Complete = true
					check.True(t, proc.Complete(ctx))
					check.Error(t, proc.RegisterTrigger(ctx, nil))
					check.Error(t, proc.RegisterTrigger(ctx, MakeDefaultTrigger(ctx, nil, &proc.info.Options, "foo")))
					check.Equal(t, len(proc.triggers), 0)
				},
				"TestRegisterPopulatedTrigger": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					check.True(t, !proc.Complete(ctx))
					check.Error(t, proc.RegisterTrigger(ctx, nil))
					check.NotError(t, proc.RegisterTrigger(ctx, MakeDefaultTrigger(ctx, nil, &proc.info.Options, "foo")))
					check.Equal(t, len(proc.triggers), 1)
				},
				"RunningIsFalseWhenCompleteIsSatisfied": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.info.Complete = true
					check.True(t, proc.Complete(ctx))
					check.True(t, !proc.Running(ctx))
				},
				"RunningIsFalseWithEmptyPid": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					signal := make(chan struct{})
					go func() {
						check.True(t, !proc.Running(ctx))
						close(signal)
					}()

					op := <-proc.ops

					op(executor.MakeLocal(&exec.Cmd{
						Process: &os.Process{},
					}))
					<-signal
				},
				"RunningIsFalseWithNilCmd": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					signal := make(chan struct{})
					go func() {
						check.True(t, !proc.Running(ctx))
						close(signal)
					}()

					op := <-proc.ops
					op(nil)

					<-signal
				},
				"RunningIsTrueWithValidPid": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					signal := make(chan struct{})
					go func() {
						check.True(t, proc.Running(ctx))
						close(signal)
					}()

					op := <-proc.ops
					op(executor.MakeLocal(&exec.Cmd{
						Process: &os.Process{Pid: 42},
					}))

					<-signal
				},
				"RunningIsFalseWithCanceledContext": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.ops <- func(_ executor.Executor) {}
					cctx, cancel := context.WithCancel(ctx)
					cancel()
					check.True(t, !proc.Running(cctx))
				},
				"SignalIsErrorAfterComplete": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.info = ProcessInfo{Complete: true}
					check.True(t, proc.Complete(ctx))

					check.Error(t, proc.Signal(ctx, syscall.SIGTERM))
				},
				"SignalNilProcessIsError": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					signal := make(chan struct{})
					go func() {
						check.True(t, !proc.Complete(ctx))
						check.Error(t, proc.Signal(ctx, syscall.SIGTERM))
						close(signal)
					}()

					op := <-proc.ops
					op(nil)

					<-signal
				},
				"SignalCanceledProcessIsError": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					cctx, cancel := context.WithCancel(ctx)
					cancel()

					check.Error(t, proc.Signal(cctx, syscall.SIGTERM))
				},
				"SignalErrorsInvalidProcess": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					signal := make(chan struct{})
					go func() {
						check.True(t, !proc.Complete(ctx))
						check.Error(t, proc.Signal(ctx, syscall.SIGTERM))
						close(signal)
					}()

					op := <-proc.ops
					op(executor.MakeLocal(&exec.Cmd{
						Process: &os.Process{Pid: -42},
					}))

					<-signal
				},
				"WaitSomeBeforeCanceling": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.info.Options = *testutil.SleepCreateOpts(10)
					proc.complete = make(chan struct{})
					cctx, cancel := context.WithTimeout(ctx, 600*time.Millisecond)
					defer cancel()

					cmd, deadline, err := proc.info.Options.Resolve(ctx)
					require.NoError(t, err)
					check.NotError(t, cmd.Start())

					go proc.reactor(ctx, deadline, cmd)
					_, err = proc.Wait(cctx)
					require.Error(t, err)
					check.Contains(t, err.Error(), "operation canceled")
				},
				"WaitShouldReturnNilForSuccessfulCommandsWithoutIDs": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.info.Options.Args = []string{"sleep", "10"}
					proc.ops = make(chan func(executor.Executor))

					cmd, _, err := proc.info.Options.Resolve(ctx)
					check.NotError(t, err)
					check.NotError(t, cmd.Start())
					signal := make(chan struct{})
					go func() {
						// this is the crucial
						// checkion of this tests
						_, err := proc.Wait(ctx)
						check.NotError(t, err)
						close(signal)
					}()

					go func() {
						for {
							select {
							case <-ctx.Done():
								grip.Warning(ctx.Err())
								return
							case op := <-proc.ops:
								proc.setInfo(ProcessInfo{
									Complete:   true,
									Successful: true,
								})
								if op != nil {
									op(cmd)
								}
							}
						}
					}()
					<-signal
				},
				"WaitShouldReturnNilForSuccessfulCommands": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.info.Options.Args = []string{"sleep", "10"}
					proc.ops = make(chan func(executor.Executor))

					cmd, _, err := proc.info.Options.Resolve(ctx)
					check.NotError(t, err)
					check.NotError(t, cmd.Start())
					signal := make(chan struct{})
					go func() {
						// this is the crucial
						// checkion of this tests
						_, err := proc.Wait(ctx)
						check.NotError(t, err)
						close(signal)
					}()

					go func() {
						for {
							select {
							case <-ctx.Done():
								grip.Warning(ctx.Err())
								return
							case op := <-proc.ops:
								proc.setInfo(ProcessInfo{
									ID:         "foo",
									Complete:   true,
									Successful: true,
								})
								if op != nil {
									op(cmd)
								}
							}
						}
					}()
					<-signal
				},
				"WaitShouldReturnErrorForFailedCommands": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					proc.info.Options.Args = []string{"sleep", "10"}
					proc.ops = make(chan func(executor.Executor))

					cmd, _, err := proc.info.Options.Resolve(ctx)
					check.NotError(t, err)
					check.NotError(t, cmd.Start())
					signal := make(chan struct{})
					go func() {
						// this is the crucial checkion
						// of this tests.
						_, err := proc.Wait(ctx)
						check.Error(t, err)
						close(signal)
					}()

					go func() {
						for {
							select {
							case <-ctx.Done():
								grip.Warning(ctx.Err())
								return
							case op := <-proc.ops:
								proc.err = errors.New("signal: killed")
								proc.setInfo(ProcessInfo{
									ID:         "foo",
									Complete:   true,
									Successful: false,
								})
								if op != nil {
									op(cmd)
								}
							}
						}
					}()
					<-signal
				},
				"InfoDoesNotWaitForContextTimeoutAfterProcessCompletes": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					opts := &options.Create{
						Args: []string{"ls"},
					}

					process, err := NewBlockingProcess(ctx, opts)
					require.NoError(t, err)

					opCompleted := make(chan struct{})

					go func() {
						defer close(opCompleted)
						_ = process.Info(ctx)
					}()

					_, err = process.Wait(ctx)
					require.NoError(t, err)

					longCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()

					select {
					case <-opCompleted:
					case <-longCtx.Done():
						check.Fail(t, "context timed out waiting for op to return")
					}
				},
				"RunningDoesNotWaitForContextTimeoutAfterProcessCompletes": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					opts := &options.Create{
						Args: []string{"ls"},
					}

					process, err := NewBlockingProcess(ctx, opts)
					require.NoError(t, err)

					opCompleted := make(chan struct{})

					go func() {
						defer close(opCompleted)
						_ = process.Running(ctx)
					}()

					_, err = process.Wait(ctx)
					require.NoError(t, err)

					longCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()

					select {
					case <-opCompleted:
					case <-longCtx.Done():
						check.Fail(t, "context timed out waiting for op to return")
					}
				},
				"SignalDoesNotWaitForContextTimeoutAfterProcessCompletes": func(ctx context.Context, t *testing.T, proc *blockingProcess) {
					opts := &options.Create{
						Args: []string{"ls"},
					}

					process, err := NewBlockingProcess(ctx, opts)
					require.NoError(t, err)

					opCompleted := make(chan struct{})

					go func() {
						defer close(opCompleted)
						_ = process.Signal(ctx, syscall.SIGKILL)
					}()

					_, _ = process.Wait(ctx)

					longCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()

					select {
					case <-opCompleted:
					case <-longCtx.Done():
						t.Error("context timed out waiting for op to return")
					}
				},
				// "": func(ctx context.Context, t *testing.T, proc *blockingProcess) {},
			} {
				t.Run(name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					id := uuid.New().String()
					proc := &blockingProcess{
						id:   id,
						ops:  make(chan func(executor.Executor), 1),
						info: ProcessInfo{ID: id},
					}

					testCase(ctx, t, proc)

					close(proc.ops)
				})
			}
		})
	}
}
