package cli

import (
	"context"
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/mock"
	"github.com/tychoish/jasper/testutil"
)

func TestSSHProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager){
		"VerifyFixture": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			assert.Equal(t, "foo", proc.info.ID)
		},
		"InfoPassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:        proc.ID(),
				IsRunning: true,
			}

			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, InfoCommand},
				&inputChecker,
				&InfoResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Info:            info,
				},
			)

			assert.Equal(t, info.ID, proc.Info(ctx).ID)
			assert.Equal(t, proc.ID(), inputChecker.ID)
		},
		"InfoWithCompletedProcessChecksInMemory": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:       proc.ID(),
				Complete: true,
			}
			proc.info = info

			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, InfoCommand},
				&inputChecker,
				&InfoResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Info:            jasper.ProcessInfo{ID: "bar"},
				},
			)

			assert.Equal(t, info.ID, proc.Info(ctx).ID)
			assert.Equal(t, len(inputChecker.ID), 0)
		},
		"RunningPassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:        proc.ID(),
				IsRunning: true,
			}

			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, RunningCommand},
				&inputChecker,
				&RunningResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Running:         info.IsRunning,
				},
			)

			assert.Equal(t, info.IsRunning, proc.Running(ctx))
			assert.Equal(t, proc.ID(), inputChecker.ID)
		},
		"RunningWithCompletedProcessChecksInMemory": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:       proc.ID(),
				Complete: true,
			}
			proc.info = info

			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, RunningCommand},
				&inputChecker,
				&RunningResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Running:         true,
				},
			)

			assert.True(t, !proc.Running(ctx))
			assert.Equal(t, len(inputChecker.ID), 0)
		},
		"CompletePassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:       proc.ID(),
				Complete: true,
			}

			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, CompleteCommand},
				&inputChecker,
				&CompleteResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Complete:        info.Complete,
				},
			)
			assert.Equal(t, info.Complete, proc.Complete(ctx))
			assert.Equal(t, proc.ID(), inputChecker.ID)
		},
		"CompleteWithCompletedProcessChecksInMemory": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:       proc.ID(),
				Complete: true,
			}
			proc.info = info

			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, CompleteCommand},
				&inputChecker,
				&CompleteResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Complete:        false,
				},
			)

			check.True(t, proc.Complete(ctx))
			assert.Equal(t, len(inputChecker.ID), 0)
		},
		"RespawnPassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:        proc.ID(),
				IsRunning: true,
			}

			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, RespawnCommand},
				&inputChecker,
				&InfoResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Info:            info,
				},
			)

			newProc, err := proc.Respawn(ctx)

			assert.NotError(t, err)
			assert.Equal(t, proc.ID(), inputChecker.ID)

			newSSHProc, ok := newProc.(*sshProcess)
			require.True(t, ok)
			assert.NotError(t, err)
			assert.Equal(t, info.ID, newSSHProc.info.ID)
		},
		"RespawnFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, RespawnCommand},
				&inputChecker,
				&struct{}{},
			)

			_, err := proc.Respawn(ctx)
			assert.Error(t, err)
			assert.Equal(t, proc.ID(), inputChecker.ID)
		},
		"SignalPassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := SignalInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, SignalCommand},
				&inputChecker,
				makeOutcomeResponse(nil),
			)

			sig := syscall.SIGINT
			assert.NotError(t, proc.Signal(ctx, sig))
			assert.Equal(t, proc.ID(), inputChecker.ID)
			assert.Equal(t, int(sig), inputChecker.Signal)
		},
		"SignalFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := SignalInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, SignalCommand},
				&inputChecker,
				&struct{}{},
			)

			sig := syscall.SIGINT
			assert.Error(t, proc.Signal(ctx, sig))
			assert.Equal(t, proc.ID(), inputChecker.ID)
			assert.Equal(t, int(sig), inputChecker.Signal)
		},
		"WaitPassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := IDInput{}
			expectedExitCode := 1
			expectedWaitErr := "foo"
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, WaitCommand},
				&inputChecker,
				&WaitResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					ExitCode:        expectedExitCode,
					Error:           expectedWaitErr,
				},
			)

			exitCode, err := proc.Wait(ctx)
			assert.Equal(t, proc.ID(), inputChecker.ID)
			require.Error(t, err)
			assert.Substring(t, err.Error(), expectedWaitErr)
			assert.Equal(t, expectedExitCode, exitCode)
		},
		"WaitFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, WaitCommand},
				&inputChecker,
				&struct{}{},
			)

			exitCode, err := proc.Wait(ctx)
			assert.Equal(t, proc.ID(), inputChecker.ID)
			assert.Error(t, err)
			assert.NotZero(t, exitCode)
		},
		"RegisterTriggerFails": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			assert.Error(t, proc.RegisterTrigger(ctx, func(jasper.ProcessInfo) {}))
		},
		"RegisterSignalTriggerFails": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			assert.Error(t, proc.RegisterSignalTrigger(ctx, func(jasper.ProcessInfo, syscall.Signal) bool { return false }))
		},
		"RegisterSignalTriggerIDPassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := SignalTriggerIDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, RegisterSignalTriggerIDCommand},
				&inputChecker,
				makeOutcomeResponse(nil),
			)

			sigID := jasper.SignalTriggerID("foo")
			assert.NotError(t, proc.RegisterSignalTriggerID(ctx, sigID))
			assert.Equal(t, proc.ID(), inputChecker.ID)
			assert.Equal(t, sigID, inputChecker.SignalTriggerID)
		},
		"TagPasses": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := TagIDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, TagCommand},
				&inputChecker,
				&struct{}{},
			)

			tag := "bar"
			proc.Tag(tag)
			assert.Equal(t, proc.ID(), inputChecker.ID)
			assert.Equal(t, tag, inputChecker.Tag)
		},
		"GetTagsPassesWithValidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := IDInput{}
			tag := "bar"
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, GetTagsCommand},
				&inputChecker,
				&TagsResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Tags:            []string{tag},
				},
			)

			tags := proc.GetTags()
			assert.Equal(t, proc.ID(), inputChecker.ID)
			assert.Equal(t, len(tags), 1)
			assert.Equal(t, tag, tags[0])
		},
		"GetTagsEmptyWithInvalidResponse": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, GetTagsCommand},
				&inputChecker,
				&TagsResponse{
					OutcomeResponse: *makeOutcomeResponse(errors.New("foo")),
				},
			)

			tags := proc.GetTags()
			assert.Equal(t, proc.ID(), inputChecker.ID)
			assert.Equal(t, len(tags), 0)
		},
		"ResetTagsPasses": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {
			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, manager,
				[]string{ProcessCommand, ResetTagsCommand},
				&inputChecker,
				&struct{}{},
			)
			proc.ResetTags()
			assert.Equal(t, proc.ID(), inputChecker.ID)
		},
		// "": func(ctx context.Context, t *testing.T, proc *sshProcess, manager *sshClient, baseManager *mock.Manager) {},
	} {
		t.Run(testName, func(t *testing.T) {
			client, err := NewSSHClient(mockRemoteOptions(), mockClientOptions(), false)
			assert.NotError(t, err)
			sshClient, ok := client.(*sshClient)
			require.True(t, ok)

			mockManager := &mock.Manager{}
			sshClient.manager = jasper.Manager(mockManager)

			tctx, cancel := context.WithTimeout(ctx, testutil.TestTimeout)
			defer cancel()

			proc, err := newSSHProcess(sshClient.runClientCommand, jasper.ProcessInfo{ID: "foo"})
			assert.NotError(t, err)
			sshProc, ok := proc.(*sshProcess)
			require.True(t, ok)

			testCase(tctx, t, sshProc, sshClient, mockManager)
		})
	}
}
