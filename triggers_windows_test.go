package jasper

import (
	"context"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func TestCleanTerminationSignalTrigger(t *testing.T) {
	for procName, makeProc := range map[string]ProcessConstructor{
		"Basic":    newBasicProcess,
		"Blocking": newBlockingProcess,
	} {
		t.Run(procName, func(t *testing.T) {
			for testName, testCase := range map[string]func(context.Context, *options.Create, ProcessConstructor){
				"CleanTerminationRunsForSIGTERM": func(ctx context.Context, opts *options.Create, makep ProcessConstructor) {
					proc, err := makep(ctx, opts)
					require.NoError(t, err)
					trigger := makeCleanTerminationSignalTrigger()
					check.True(t, trigger(proc.Info(ctx), syscall.SIGTERM))

					exitCode, err := proc.Wait(ctx)
					check.NotError(t, err)
					check.Zero(t, exitCode)
					check.True(t, !proc.Running(ctx))

					// Subsequent executions of trigger should fail.
					check.True(t, !trigger(proc.Info(ctx), syscall.SIGTERM))
				},
				"CleanTerminationIgnoresNonSIGTERM": func(ctx context.Context, opts *options.Create, makep ProcessConstructor) {
					proc, err := makep(ctx, opts)
					require.NoError(t, err)
					trigger := makeCleanTerminationSignalTrigger()
					check.True(t, !trigger(proc.Info(ctx), syscall.SIGHUP))

					check.True(t, proc.Running(ctx))

					check.NotError(t, proc.Signal(ctx, syscall.SIGKILL))
				},
				"CleanTerminationFailsForExitedProcess": func(ctx context.Context, opts *options.Create, makep ProcessConstructor) {
					opts = testutil.TrueCreateOpts()
					proc, err := makep(ctx, opts)
					require.NoError(t, err)

					exitCode, err := proc.Wait(ctx)
					check.NotError(t, err)
					check.Zero(t, exitCode)

					trigger := makeCleanTerminationSignalTrigger()
					check.True(t, !trigger(proc.Info(ctx), syscall.SIGTERM))
				},
			} {
				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()

					testCase(ctx, testutil.SleepCreateOpts(1), makeProc)
				})
			}
		})
	}
}
