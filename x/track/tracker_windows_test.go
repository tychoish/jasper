//go:build windows
// +build windows

package track

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func makeTracker() (*windowsProcessTracker, error) {
	tracker, err := NewProcessTracker("foo" + uuid.New().String())
	if err != nil {
		return nil, err
	}

	windowsTracker, ok := tracker.(*windowsProcessTracker)
	if !ok {
		return nil, errors.New("not a Windows process tracker")
	}
	return windowsTracker, nil
}

func TestWindowsProcessTracker(t *testing.T) {
	for testName, testCase := range map[string]func(context.Context, *testing.T, *windowsProcessTracker, *options.Create){
		"NewWindowsProcessTrackerCreatesJob": func(_ context.Context, t *testing.T, tracker *windowsProcessTracker, opts *options.Create) {
			assert.True(t, tracker.job != nil)
			info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
			check.NotError(t, err)
			check.Equal(t, 0, int(info.NumberOfAssignedProcesses))
		},
		"AddProcessToTrackerAssignsPID": func(ctx context.Context, t *testing.T, tracker *windowsProcessTracker, opts *options.Create) {
			opts1, opts2 := opts, opts.Copy()
			proc1, err := newBasicProcess(ctx, opts1)
			assert.NotError(t, err)
			check.NotError(t, tracker.Add(proc1.Info(ctx)))

			proc2, err := newBasicProcess(ctx, opts2)
			assert.NotError(t, err)
			check.NotError(t, tracker.Add(proc2.Info(ctx)))

			info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
			check.NotError(t, err)
			check.Equal(t, 2, int(info.NumberOfAssignedProcesses))
			check.Contains(t, info.ProcessIdList, uint64(proc1.Info(ctx).PID))
			check.Contains(t, info.ProcessIdList, uint64(proc2.Info(ctx).PID))
		},
		"AddedProcessIsTerminatedOnCleanup": func(ctx context.Context, t *testing.T, tracker *windowsProcessTracker, opts *options.Create) {
			proc, err := newBasicProcess(ctx, opts)
			assert.NotError(t, err)

			check.NotError(t, tracker.Add(proc.Info(ctx)))

			info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
			check.NotError(t, err)
			check.Equal(t, 1, int(info.NumberOfAssignedProcesses))
			check.Contains(t, info.ProcessIdList, uint64(proc.Info(ctx).PID))

			check.NotError(t, tracker.Cleanup())

			exitCode, err := proc.Wait(ctx)
			check.Zero(t, exitCode)
			check.NotError(t, err)
			check.NotError(t, ctx.Err())
			check.True(t, proc.Complete(ctx))
		},
		"CleanupWithNoProcessesDoesNotError": func(ctx context.Context, t *testing.T, tracker *windowsProcessTracker, opts *options.Create) {
			check.NotError(t, tracker.Cleanup())
		},
		"DoubleCleanupDoesNotError": func(ctx context.Context, t *testing.T, tracker *windowsProcessTracker, opts *options.Create) {
			proc, err := newBasicProcess(ctx, opts)
			assert.NotError(t, err)

			check.NotError(t, tracker.Add(proc.Info(ctx)))

			info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
			check.NotError(t, err)
			check.Equal(t, 1, int(info.NumberOfAssignedProcesses))
			check.Contains(t, info.ProcessIdList, uint64(proc.Info(ctx).PID))

			check.NotError(t, tracker.Cleanup())
			check.NotError(t, tracker.Cleanup())

			exitCode, err := proc.Wait(ctx)
			check.Zero(t, exitCode)
			check.NotError(t, err)
			check.NotError(t, ctx.Err())
			check.True(t, proc.Complete(ctx))
		},
		"CanAddProcessAfterCleanup": func(ctx context.Context, t *testing.T, tracker *windowsProcessTracker, opts *options.Create) {
			check.NotError(t, tracker.Cleanup())

			proc, err := newBasicProcess(ctx, opts)
			assert.NotError(t, err)

			check.NotError(t, tracker.Add(proc.Info(ctx)))
			info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
			check.NotError(t, err)
			check.Equal(t, 1, int(info.NumberOfAssignedProcesses))
		},
		// "": func(ctx context.Context, t *testing.T, tracker *windowsProcessTracker) {},
	} {
		t.Run(testName, func(t *testing.T) {
			if _, runningInEvgAgent := os.LookupEnv("EVR_TASK_ID"); runningInEvgAgent {
				t.Skip("Evergreen makes its own job object, so these will not pass in Evergreen tests ",
					"(although they will pass if locally run).")
			}
			ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
			defer cancel()

			tracker, err := makeTracker()
			defer func() {
				check.NotError(t, tracker.Cleanup())
			}()
			assert.NotError(t, err)
			assert.True(t, tracker != nil)

			testCase(ctx, t, tracker, testutil.SleepCreateOpts(1))
		})
	}
}
