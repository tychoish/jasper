//go:build windows
// +build windows

package jasper

import (
	"context"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func TestBasicManagerWithTrackedProcesses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for managerName, makeManager := range map[string]func(ctx context.Context, t *testing.T) *basicProcessManager{
		"Basic": func(ctx context.Context, t *testing.T) *basicProcessManager {
			basicManager, err := newBasicProcessManager(map[string]Process{}, true, false)
			require.NoError(t, err)
			return basicManager.(*basicProcessManager)
		},
	} {
		t.Run(managerName, func(t *testing.T) {
			for testName, testCase := range map[string]func(context.Context, *testing.T, *basicProcessManager, *windowsProcessTracker, *options.Create){
				"ProcessTrackerCreatedEmpty": func(_ context.Context, t *testing.T, m *basicProcessManager, tracker *windowsProcessTracker, _ *options.Create) {
					require.NotNil(t, tracker.job)

					info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
					check.NotError(t, err)
					check.Zero(t, info.NumberOfAssignedProcesses)
				},
				"CreateAddsProcess": func(ctx context.Context, t *testing.T, m *basicProcessManager, tracker *windowsProcessTracker, opts *options.Create) {
					proc, err := m.CreateProcess(ctx, opts)
					require.NoError(t, err)

					info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
					check.NotError(t, err)
					check.Equal(t, 1, int(info.NumberOfAssignedProcesses))
					check.Equal(t, proc.Info(ctx).PID, int(info.ProcessIdList[0]))
					check.NotError(t, m.Close(ctx))
				},
				"RegisterAddsProcess": func(ctx context.Context, t *testing.T, m *basicProcessManager, tracker *windowsProcessTracker, opts *options.Create) {
					proc, err := newBasicProcess(ctx, opts)
					require.NoError(t, err)
					check.NotError(t, m.Register(ctx, proc))

					info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
					check.NotError(t, err)
					check.Equal(t, 1, int(info.NumberOfAssignedProcesses))
					check.Equal(t, proc.Info(ctx).PID, int(info.ProcessIdList[0]))
					check.NotError(t, m.Close(ctx))
				},
				"ClosePerformsProcessTrackingCleanup": func(ctx context.Context, t *testing.T, m *basicProcessManager, tracker *windowsProcessTracker, opts *options.Create) {
					proc, err := m.CreateProcess(ctx, opts)
					require.NoError(t, err)

					info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
					check.NotError(t, err)
					check.Equal(t, 1, int(info.NumberOfAssignedProcesses))
					check.Equal(t, proc.Info(ctx).PID, int(info.ProcessIdList[0]))
					check.NotError(t, m.Close(ctx))

					exitCode, err := proc.Wait(ctx)
					check.NotError(t, err)
					check.Zero(t, exitCode)
					check.True(t, !proc.Running(ctx))
					check.True(t, proc.Complete(ctx))
				},
				"CloseOnTerminatedProcessSucceeds": func(ctx context.Context, t *testing.T, m *basicProcessManager, tracker *windowsProcessTracker, opts *options.Create) {
					proc, err := m.CreateProcess(ctx, opts)
					require.NoError(t, err)

					info, err := QueryInformationJobObjectProcessIdList(tracker.job.handle)
					check.NotError(t, err)
					check.Equal(t, 1, int(info.NumberOfAssignedProcesses))
					check.Equal(t, proc.Info(ctx).PID, int(info.ProcessIdList[0]))

					check.NotError(t, proc.Signal(ctx, syscall.SIGKILL))
					check.NotError(t, m.Close(ctx))
				},
			} {
				t.Run(testName, func(t *testing.T) {
					if _, runningInEvgAgent := os.LookupEnv("EVR_TASK_ID"); runningInEvgAgent {
						t.Skip("Evergreen makes its own job object, so these will not pass in Evergreen tests ",
							"(although they will pass if locally run).")
					}
					tctx, cancel := context.WithTimeout(ctx, testutil.TestTimeout)
					defer cancel()
					manager := makeManager(tctx, t)
					tracker, ok := manager.tracker.(*windowsProcessTracker)
					require.True(t, ok)
					testCase(tctx, t, manager, tracker, testutil.SleepCreateOpts(1))
				})
			}
		})
	}
}
