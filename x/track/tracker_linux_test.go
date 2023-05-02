//go:build linux
// +build linux

package track

import (
	"context"
	"os"
	"syscall"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func TestLinuxProcessTrackerWithCgroups(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("cannot run Linux process tracker tests with cgroups without admin privileges")
	}
	for procName, makeProc := range map[string]jasper.ProcessConstructor{
		"Blocking": jasper.NewBlockingProcess,
		"Basic":    jasper.NewBasicProcess,
	} {
		t.Run(procName, func(t *testing.T) {

			for name, testCase := range map[string]func(context.Context, *testing.T, *linuxProcessTracker, jasper.Process){
				"VerifyInitialSetup": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					assert.True(t, tracker.cgroup != nil)
					assert.True(t, tracker.validCgroup())
					pids, err := tracker.listCgroupPIDs()
					assert.NotError(t, err)
					check.Equal(t, len(pids), 0)
				},
				"NilCgroupIsInvalid": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					tracker.cgroup = nil
					check.True(t, !tracker.validCgroup())
				},
				"DeletedCgroupIsInvalid": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					assert.NotError(t, tracker.cgroup.Delete())
					check.True(t, !tracker.validCgroup())
				},
				"SetDefaultCgroupIfInvalidNoopsIfCgroupIsValid": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					cgroup := tracker.cgroup
					check.True(t, cgroup != nil)
					check.NotError(t, tracker.setDefaultCgroupIfInvalid())
					check.True(t, cgroup == tracker.cgroup)
				},
				"SetDefaultCgroupIfNilSetsIfCgroupIsInvalid": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					tracker.cgroup = nil
					check.NotError(t, tracker.setDefaultCgroupIfInvalid())
					check.True(t, tracker.cgroup != nil)
				},
				"AddNewProcessSucceeds": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					check.NotError(t, tracker.Add(proc.Info(ctx)))

					pids, err := tracker.listCgroupPIDs()
					assert.NotError(t, err)
					check.Equal(t, len(pids), 1)
					check.Contains(t, pids, proc.Info(ctx).PID)
				},
				"DoubleAddProcessSucceedsButDoesNotDuplicateProcessInCgroup": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					pid := proc.Info(ctx).PID
					check.NotError(t, tracker.Add(proc.Info(ctx)))
					check.NotError(t, tracker.Add(proc.Info(ctx)))

					pids, err := tracker.listCgroupPIDs()
					assert.NotError(t, err)
					check.Equal(t, len(pids), 1)
					check.Contains(t, pids, pid)
				},
				"ListCgroupPIDsDoesNotSeeTerminatedProcesses": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					assert.NotError(t, tracker.Add(proc.Info(ctx)))

					check.NotError(t, proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger))
					err := proc.Signal(ctx, syscall.SIGTERM)
					check.NotError(t, err)
					exitCode, err := proc.Wait(ctx)
					check.Error(t, err)
					check.Equal(t, exitCode, int(syscall.SIGTERM))

					pids, err := tracker.listCgroupPIDs()
					assert.NotError(t, err)
					check.Equal(t, len(pids), 0)
				},
				"ListCgroupPIDsDoesNotErrorIfCgroupDeleted": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					check.NotError(t, tracker.cgroup.Delete())
					pids, err := tracker.listCgroupPIDs()
					check.NotError(t, err)
					check.Equal(t, len(pids), 0)
				},
				"CleanupNoProcsSucceeds": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					pids, err := tracker.listCgroupPIDs()
					assert.NotError(t, err)
					check.Equal(t, len(pids), 0)
					check.NotError(t, tracker.Cleanup())
				},
				"CleanupTerminatesProcessInCgroup": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					check.NotError(t, tracker.Add(proc.Info(ctx)))
					check.NotError(t, tracker.Cleanup())

					procTerminated := make(chan struct{})
					go func() {
						defer close(procTerminated)
						_, _ = proc.Wait(ctx)
					}()

					select {
					case <-procTerminated:
					case <-ctx.Done():
						t.Error("context timed out before process was complete")
					}
				},
				"CleanupAfterDoubleAddDoesNotError": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					check.NotError(t, tracker.Add(proc.Info(ctx)))
					check.NotError(t, tracker.Add(proc.Info(ctx)))
					check.NotError(t, tracker.Cleanup())
				},
				"DoubleCleanupDoesNotError": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					check.NotError(t, tracker.Add(proc.Info(ctx)))
					check.NotError(t, tracker.Cleanup())
					check.NotError(t, tracker.Cleanup())
				},
				"AddProcessAfterCleanupSucceeds": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, proc jasper.Process) {
					assert.NotError(t, tracker.Add(proc.Info(ctx)))
					assert.NotError(t, tracker.Cleanup())

					newProc, err := makeProc(ctx, testutil.SleepCreateOpts(1))
					assert.NotError(t, err)

					assert.NotError(t, tracker.Add(newProc.Info(ctx)))
					pids, err := tracker.listCgroupPIDs()
					assert.NotError(t, err)
					check.Equal(t, len(pids), 1)
					check.Contains(t, pids, newProc.Info(ctx).PID)
				},
			} {
				t.Run(name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()

					opts := testutil.SleepCreateOpts(1)
					proc, err := makeProc(ctx, opts)
					assert.NotError(t, err)

					tracker, err := New("test")
					assert.NotError(t, err)
					assert.True(t, tracker != nil)
					linuxTracker, ok := tracker.(*linuxProcessTracker)
					assert.True(t, ok)
					defer func() {
						// Ensure that the cgroup is cleaned up.
						check.NotError(t, tracker.Cleanup())
					}()

					testCase(ctx, t, linuxTracker, proc)
				})
			}
		})
	}
}

func TestLinuxProcessTrackerWithEnvironmentVariables(t *testing.T) {
	for procName, makeProc := range map[string]jasper.ProcessConstructor{
		"Blocking": jasper.NewBlockingProcess,
		"Basic":    jasper.NewBasicProcess,
	} {
		t.Run(procName, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, opts *options.Create, envVarName string, envVarValue string){
				"CleanupFindsProcessesByEnvironmentVariable": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, opts *options.Create, envVarName string, envVarValue string) {
					opts.AddEnvVar(envVarName, envVarValue)
					proc, err := makeProc(ctx, opts)
					assert.NotError(t, err)
					check.NotError(t, tracker.Add(proc.Info(ctx)))
					// Cgroup might be re-initialized in Add(), so invalidate
					// it.
					tracker.cgroup = nil
					check.NotError(t, tracker.Cleanup())

					procTerminated := make(chan struct{})
					go func() {
						defer close(procTerminated)
						_, _ = proc.Wait(ctx)
					}()

					select {
					case <-procTerminated:
					case <-ctx.Done():
						t.Error("context timed out before process was complete")
					}
				},
				"CleanupIgnoresAddedProcessesWithoutEnvironmentVariable": func(ctx context.Context, t *testing.T, tracker *linuxProcessTracker, opts *options.Create, envVarName string, envVarValue string) {
					proc, err := makeProc(ctx, opts)
					assert.NotError(t, err)
					_, ok := proc.Info(ctx).Options.Environment[envVarName]
					assert.True(t, !ok)

					check.NotError(t, tracker.Add(proc.Info(ctx)))
					// Cgroup might be re-initialized in Add(), so invalidate
					// it.
					tracker.cgroup = nil
					check.NotError(t, tracker.Cleanup())
					check.True(t, proc.Running(ctx))
				},
				// "": func(ctx, context.Context, t *testing.T, tracker *linuxProcessTracker, envVarName string, envVarValue string) {},
			} {
				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()

					envVarValue := "bar"

					tracker, err := New(envVarValue)
					assert.NotError(t, err)
					assert.True(t, tracker != nil)
					linuxTracker, ok := tracker.(*linuxProcessTracker)
					assert.True(t, ok)
					defer func() {
						// Ensure that the cgroup is cleaned up.
						check.NotError(t, tracker.Cleanup())
					}()
					// Override default cgroup behavior.
					linuxTracker.cgroup = nil

					testCase(ctx, t, linuxTracker, testutil.SleepCreateOpts(1), jasper.ManagerEnvironID, envVarValue)
				})
			}
		})
	}
}
