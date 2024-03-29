package testcases

import (
	"bytes"
	"context"
	"os"
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

func ProcessConstructors() map[string]jasper.ProcessConstructor {
	return map[string]jasper.ProcessConstructor{
		"BlockingNoLock":   jasper.NewBlockingProcess,
		"BlockingWithLock": makeLockingProcess(jasper.NewBlockingProcess),
		"BasicNoLock":      jasper.NewBasicProcess,
		"BasicWithLock":    makeLockingProcess(jasper.NewBasicProcess),
	}
}

type ProcessCase func(context.Context, *testing.T, *options.Create, jasper.ProcessConstructor)

func ProcessCases() map[string]ProcessCase {
	return map[string]ProcessCase{
		"WithPopulatedArgsCommandCreationPasses": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			check.NotEqual(t, len(opts.Args), 0)
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			check.True(t, proc != nil)
		},
		"ErrorToCreateWithInvalidArgs": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			opts.Args = []string{}
			proc, err := makep(ctx, opts)
			check.Error(t, err)
			check.True(t, proc == nil)
		},
		"WithCanceledContextProcessCreationFails": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			pctx, pcancel := context.WithCancel(ctx)
			pcancel()
			proc, err := makep(pctx, opts)
			check.Error(t, err)
			check.True(t, proc == nil)
		},
		"CanceledContextTimesOutEarly": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
			defer pcancel()
			startAt := time.Now()
			opts := testutil.SleepCreateOpts(20)
			proc, err := makep(pctx, opts)
			assert.NotError(t, err)
			assert.True(t, proc != nil)

			time.Sleep(5 * time.Millisecond) // let time pass...
			check.True(t, !proc.Info(ctx).Successful)
			check.True(t, time.Since(startAt) < 20*time.Second)
		},
		"ProcessLacksTagsByDefault": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			tags := proc.GetTags()
			check.Equal(t, len(tags), 0)
		},
		"ProcessTagsPersist": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			opts.Tags = []string{"foo"}
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			tags := proc.GetTags()
			check.Contains(t, tags, "foo")
		},
		"InfoTagsMatchGetTags": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			opts.Tags = []string{"foo"}
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			tags := proc.GetTags()
			check.Contains(t, tags, "foo")
			check.EqualItems(t, tags, proc.Info(ctx).Options.Tags)

			proc.ResetTags()
			tags = proc.GetTags()
			check.Equal(t, 0, len(tags))
			check.Equal(t, 0, len(proc.Info(ctx).Options.Tags))
		},
		"InfoHasMatchingID": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			_, err = proc.Wait(ctx)
			assert.NotError(t, err)
			check.Equal(t, proc.ID(), proc.Info(ctx).ID)
		},
		"ResetTags": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			proc.Tag("foo")
			check.Contains(t, proc.GetTags(), "foo")
			proc.ResetTags()
			check.Equal(t, len(proc.GetTags()), 0)
		},
		"TagsAreSetLike": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)

			for i := 0; i < 10; i++ {
				proc.Tag("foo")
			}

			check.Equal(t, len(proc.GetTags()), 1)
			proc.Tag("bar")
			check.Equal(t, len(proc.GetTags()), 2)
		},
		"CompleteIsTrueAfterWait": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			time.Sleep(10 * time.Millisecond) // give the process time to start background machinery
			_, err = proc.Wait(ctx)
			check.NotError(t, err)
			check.True(t, proc.Complete(ctx))
		},
		"WaitReturnsWithCanceledContext": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			opts.Args = []string{"sleep", "20"}
			pctx, pcancel := context.WithCancel(ctx)
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			check.True(t, proc.Running(ctx))
			check.NotError(t, err)
			pcancel()
			_, err = proc.Wait(pctx)
			check.Error(t, err)
		},
		"RegisterTriggerErrorsForNil": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			check.Error(t, proc.RegisterTrigger(ctx, nil))
		},
		"RegisterSignalTriggerErrorsForNil": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			check.Error(t, proc.RegisterSignalTrigger(ctx, nil))
		},
		"RegisterSignalTriggerErrorsForExitedProcess": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			_, err = proc.Wait(ctx)
			check.NotError(t, err)
			check.Error(t, proc.RegisterSignalTrigger(ctx, func(_ jasper.ProcessInfo, _ syscall.Signal) bool { return false }))
		},
		"RegisterSignalTriggerIDErrorsForExitedProcess": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			_, err = proc.Wait(ctx)
			check.NotError(t, err)
			check.Error(t, proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger))
		},
		"RegisterSignalTriggerIDFailsWithInvalidTriggerID": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			opts := testutil.SleepCreateOpts(3)
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			check.Error(t, proc.RegisterSignalTriggerID(ctx, jasper.SignalTriggerID("foo")))
		},
		"RegisterSignalTriggerIDPassesWithValidTriggerID": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			opts := testutil.SleepCreateOpts(3)
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			check.NotError(t, proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger))
		},
		"DefaultTriggerSucceeds": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			opts := testutil.SleepCreateOpts(3)
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			check.NotError(t, proc.RegisterTrigger(ctx, jasper.MakeDefaultTrigger(ctx, nil, opts, "foo")))
		},
		"OptionsCloseTriggerRegisteredByDefault": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			count := 0
			countIncremented := make(chan bool, 1)
			opts.RegisterCloser(func() (_ error) {
				count++
				countIncremented <- true
				close(countIncremented)
				return
			})

			proc, err := makep(ctx, opts)
			assert.NotError(t, err)

			_, err = proc.Wait(ctx)
			assert.NotError(t, err)

			select {
			case <-ctx.Done():
				t.Fatal("closers took too long to run")
			case <-countIncremented:
				check.Equal(t, 1, count)
			}
		},
		"SignalTriggerRunsBeforeSignal": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, testutil.SleepCreateOpts(1))
			assert.NotError(t, err)

			expectedSig := syscall.SIGKILL
			check.NotError(t, proc.RegisterSignalTrigger(ctx, func(info jasper.ProcessInfo, actualSig syscall.Signal) bool {
				check.Equal(t, expectedSig, actualSig)
				check.True(t, info.IsRunning)
				check.True(t, !info.Complete)
				return false
			}))
			check.NotError(t, proc.Signal(ctx, expectedSig))

			exitCode, err := proc.Wait(ctx)
			check.Error(t, err)
			if runtime.GOOS == "windows" {
				check.Equal(t, 1, exitCode)
			} else {
				check.Equal(t, int(expectedSig), exitCode)
			}

			check.True(t, !proc.Running(ctx))
			check.True(t, proc.Complete(ctx))
		},
		"SignalTriggerCanSkipSignal": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, testutil.SleepCreateOpts(1))
			assert.NotError(t, err)

			expectedSig := syscall.SIGKILL
			shouldSkipNextTime := true
			check.NotError(t, proc.RegisterSignalTrigger(ctx, func(info jasper.ProcessInfo, actualSig syscall.Signal) bool {
				check.Equal(t, expectedSig, actualSig)
				skipSignal := shouldSkipNextTime
				shouldSkipNextTime = false
				return skipSignal
			}))

			check.NotError(t, proc.Signal(ctx, expectedSig))
			check.True(t, proc.Running(ctx))
			check.True(t, !proc.Complete(ctx))

			check.NotError(t, proc.Signal(ctx, expectedSig))

			exitCode, err := proc.Wait(ctx)
			check.Error(t, err)
			if runtime.GOOS == "windows" {
				check.Equal(t, 1, exitCode)
			} else {
				check.Equal(t, int(expectedSig), exitCode)
			}

			check.True(t, !proc.Running(ctx))
			check.True(t, proc.Complete(ctx))
		},
		"ProcessLogDefault": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			file, err := os.CreateTemp(testutil.BuildDirectory(), "out.txt")
			assert.NotError(t, err)
			defer func() {
				check.NotError(t, file.Close())
				check.NotError(t, os.RemoveAll(file.Name()))
			}()
			info, err := file.Stat()
			assert.NotError(t, err)
			check.Zero(t, info.Size())

			logger := &options.LoggerConfig{}
			assert.NotError(t, logger.Set(&options.DefaultLoggerOptions{
				Base: options.BaseOptions{Format: options.LogFormatPlain},
			}))
			opts.Output.Loggers = []*options.LoggerConfig{logger}
			opts.Args = []string{"echo", "foobar"}

			proc, err := makep(ctx, opts)
			assert.NotError(t, err)

			_, err = proc.Wait(ctx)
			check.NotError(t, err)
		},
		"ProcessWritesToLog": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			file, err := os.CreateTemp(testutil.BuildDirectory(), "out.txt")
			assert.NotError(t, err)
			defer func() {
				check.NotError(t, file.Close())
				check.NotError(t, os.RemoveAll(file.Name()))
			}()
			info, err := file.Stat()
			assert.NotError(t, err)
			check.Zero(t, info.Size())

			logger := &options.LoggerConfig{}
			assert.NotError(t, logger.Set(&options.FileLoggerOptions{
				Filename: file.Name(),
				Base:     options.BaseOptions{Format: options.LogFormatPlain},
			}))
			opts.Output.Loggers = []*options.LoggerConfig{logger}
			opts.Args = []string{"echo", "foobar"}

			proc, err := makep(ctx, opts)
			assert.NotError(t, err)

			_, err = proc.Wait(ctx)
			check.NotError(t, err)

			// File is not guaranteed to be written once Wait() returns and closers begin executing,
			// so wait for file to be non-empty.
			fileWrite := make(chan bool)
			go func() {
				done := false
				for !done {
					info, err = file.Stat()
					check.NotError(t, err)
					if info.Size() > 0 {
						done = true
						fileWrite <- done
					}
				}
			}()

			select {
			case <-ctx.Done():
				t.Error("file write took too long to complete")
			case <-fileWrite:
				info, err = file.Stat()
				assert.NotError(t, err)
				check.NotZero(t, info.Size())
			}
		},
		"WaitOnRespawnedProcessDoesNotError": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
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
		"RespawnedProcessGivesSameResult": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
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
			check.Equal(t, procExitCode, proc.Info(ctx).ExitCode)
		},
		"RespawningFinishedProcessIsOK": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			assert.True(t, proc != nil)
			_, err = proc.Wait(ctx)
			assert.NotError(t, err)

			newProc, err := proc.Respawn(ctx)
			assert.NotError(t, err)
			assert.True(t, newProc != nil)
			_, err = newProc.Wait(ctx)
			assert.NotError(t, err)
			check.True(t, newProc.Info(ctx).Successful)
		},
		"RespawningRunningProcessIsOK": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			opts := testutil.SleepCreateOpts(2)
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			assert.True(t, proc != nil)

			newProc, err := proc.Respawn(ctx)
			assert.NotError(t, err)
			assert.True(t, newProc != nil)
			_, err = newProc.Wait(ctx)
			assert.NotError(t, err)
			check.True(t, newProc.Info(ctx).Successful)
		},
		"TriggersFireOnRespawnedProcessExit": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			count := 0
			opts := testutil.SleepCreateOpts(2)
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)
			assert.True(t, proc != nil)

			countIncremented := make(chan bool)
			check.NotError(t, proc.RegisterTrigger(ctx, func(pInfo jasper.ProcessInfo) {
				count++
				countIncremented <- true
			}))
			time.Sleep(3 * time.Second)

			select {
			case <-ctx.Done():
				t.Error("triggers took too long to run")
			case <-countIncremented:
				assert.Equal(t, 1, count)
			}

			newProc, err := proc.Respawn(ctx)
			assert.NotError(t, err)
			assert.True(t, newProc != nil)
			check.NotError(t, newProc.RegisterTrigger(ctx, func(pIfno jasper.ProcessInfo) {
				count++
				countIncremented <- true
			}))
			time.Sleep(3 * time.Second)

			select {
			case <-ctx.Done():
				t.Error("triggers took too long to run")
			case <-countIncremented:
				check.Equal(t, 2, count)
			}
		},
		"RespawnShowsConsistentStateValues": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			opts := testutil.SleepCreateOpts(2)
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
			check.True(t, newProc.Complete(ctx))
		},
		"WaitGivesSuccessfulExitCode": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, testutil.TrueCreateOpts())
			assert.NotError(t, err)
			assert.True(t, proc != nil)
			exitCode, err := proc.Wait(ctx)
			check.NotError(t, err)
			check.Equal(t, 0, exitCode)
		},
		"WaitGivesFailureExitCode": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, testutil.FalseCreateOpts())
			assert.NotError(t, err)
			assert.True(t, proc != nil)
			exitCode, err := proc.Wait(ctx)
			check.Error(t, err)
			check.Equal(t, 1, exitCode)
		},
		"WaitGivesProperExitCodeOnSignalDeath": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, testutil.SleepCreateOpts(100))
			assert.NotError(t, err)
			assert.True(t, proc != nil)
			sig := syscall.SIGTERM
			check.NotError(t, proc.Signal(ctx, sig))
			exitCode, err := proc.Wait(ctx)
			check.Error(t, err)
			if runtime.GOOS == "windows" {
				check.Equal(t, 1, exitCode)
			} else {
				check.Equal(t, int(sig), exitCode)
			}
		},
		"WaitGivesProperExitCodeOnSignalAbort": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, testutil.SleepCreateOpts(100))
			assert.NotError(t, err)
			assert.True(t, proc != nil)
			sig := syscall.SIGABRT
			check.NotError(t, proc.Signal(ctx, sig))
			exitCode, err := proc.Wait(ctx)
			check.Error(t, err)
			if runtime.GOOS == "windows" {
				check.Equal(t, 1, exitCode)
			} else {
				check.Equal(t, int(sig), exitCode)
			}
		},
		"WaitGivesNegativeOneOnAlternativeError": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
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
				check.Error(t, err)
				check.Equal(t, -1, exitCode)
			case <-ctx.Done():
				t.Error("call to Wait() took too long to finish")
			}
		},
		"InfoHasTimeoutWhenProcessTimesOut": func(ctx context.Context, t *testing.T, _ *options.Create, makep jasper.ProcessConstructor) {
			opts := testutil.SleepCreateOpts(100)
			opts.Timeout = time.Second
			opts.TimeoutSecs = 1
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)

			exitCode, err := proc.Wait(ctx)
			check.Error(t, err)
			if runtime.GOOS == "windows" {
				check.Equal(t, 1, exitCode)
			} else {
				check.Equal(t, int(syscall.SIGKILL), exitCode)
			}
			check.True(t, proc.Info(ctx).Timeout)
		},
		"CallingSignalOnDeadProcessDoesError": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			proc, err := makep(ctx, opts)
			assert.NotError(t, err)

			_, err = proc.Wait(ctx)
			check.NotError(t, err)

			err = proc.Signal(ctx, syscall.SIGTERM)
			assert.Error(t, err)
			check.True(t, strings.Contains(err.Error(), "cannot signal a process that has terminated"))
		},
		"StandardInput": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {
			for subTestName, subTestCase := range map[string]func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte, output *bytes.Buffer){
				"ReaderSetsProcessStandardInput": func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte, output *bytes.Buffer) {
					opts.StandardInput = bytes.NewBuffer(stdin)

					proc, err := makep(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					check.Equal(t, expectedOutput, strings.TrimSpace(output.String()))
				},
				"BytesSetsProcessStandardInput": func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte, output *bytes.Buffer) {
					opts.StandardInputBytes = stdin

					proc, err := makep(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					check.Equal(t, expectedOutput, strings.TrimSpace(output.String()))
				},
				"ReaderNotRereadByRespawn": func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte, output *bytes.Buffer) {
					opts.StandardInput = bytes.NewBuffer(stdin)

					proc, err := makep(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					check.Equal(t, expectedOutput, strings.TrimSpace(output.String()))

					output.Reset()

					newProc, err := proc.Respawn(ctx)
					assert.NotError(t, err)

					_, err = newProc.Wait(ctx)
					assert.NotError(t, err)

					check.Zero(t, len(output.String()))

					check.True(t, proc.Info(ctx).Options.StandardInput == newProc.Info(ctx).Options.StandardInput)
				},
				"BytesCopiedByRespawn": func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte, output *bytes.Buffer) {
					opts.StandardInputBytes = stdin

					proc, err := makep(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					check.Equal(t, expectedOutput, strings.TrimSpace(output.String()))

					output.Reset()

					newProc, err := proc.Respawn(ctx)
					assert.NotError(t, err)

					_, err = newProc.Wait(ctx)
					assert.NotError(t, err)

					check.Equal(t, expectedOutput, strings.TrimSpace(output.String()))
				},
			} {
				t.Run(subTestName, func(t *testing.T) {
					output := &bytes.Buffer{}
					opts = &options.Create{
						Args: []string{"bash", "-s"},
						Output: options.Output{
							Output: output,
						},
					}
					expectedOutput := "foobar"
					stdin := []byte("echo " + expectedOutput)
					subTestCase(ctx, t, opts, expectedOutput, stdin, output)
				})
			}
		},
		// "": func(ctx context.Context, t *testing.T, opts *options.Create, makep jasper.ProcessConstructor) {},
	}
}
