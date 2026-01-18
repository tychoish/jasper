package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	"github.com/urfave/cli/v3"
)

func tagProcess(t *testing.T, jasperProcID string, tag string) OutcomeResponse {
	input, err := json.Marshal(TagIDInput{ID: jasperProcID, Tag: tag})
	assert.NotError(t, err)
	resp := OutcomeResponse{}
	assert.NotError(t, execCLICommandInputOutput(t, processTag(), []string{string(input)}, &resp))
	return resp
}

const nonexistentID = "nonexistent"

func TestCLIProcess(t *testing.T) {
	for remoteType, makeService := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) util.CloseFunc{
		RESTService: makeTestRESTService,
		RPCService:  makeTestRPCService,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string){
				"InfoWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processInfo(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, jasperProcID, resp.Info.ID)
					check.True(t, resp.Info.IsRunning)
				},
				"InfoWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processInfo(), []string{string(input)}, resp))
					assert.True(t, !resp.Successful())
				},
				"InfoWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, processInfo(), []string{string(input)}, &InfoResponse{}))
				},
				"RunningWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &RunningResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processRunning(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					check.True(t, resp.Running)
				},
				"RunningWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &RunningResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processRunning(), []string{string(input)}, resp))
					assert.True(t, !resp.Successful())
				},
				"RunningWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, processRunning(), []string{string(input)}, &RunningResponse{}))
				},
				"CompleteWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &CompleteResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processComplete(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					assert.True(t, !resp.Complete)
				},
				"CompleteWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &CompleteResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processComplete(), []string{string(input)}, resp))
					assert.True(t, !resp.Successful())
				},
				"CompleteWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, processComplete(), []string{string(input)}, &CompleteResponse{}))
				},
				"WaitWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &WaitResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processWait(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.Error), 0)
					assert.Zero(t, resp.ExitCode)
				},
				"WaitWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &WaitResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processWait(), []string{string(input)}, resp))
					assert.True(t, !resp.Successful())
				},
				"WaitWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, processWait(), []string{string(input)}, &WaitResponse{}))
				},
				"Respawn": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processRespawn(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					assert.NotZero(t, resp.Info.ID)
					assert.NotEqual(t, resp.Info.ID, jasperProcID)
				},
				"RegisterSignalTriggerID": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(SignalTriggerIDInput{ID: jasperProcID, SignalTriggerID: jasper.CleanTerminationSignalTrigger})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processRegisterSignalTriggerID(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
				},
				"Tag": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					assert.True(t, tagProcess(t, jasperProcID, "foo").Successful())
				},
				"TagNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					assert.True(t, !tagProcess(t, nonexistentID, "foo").Successful())
				},
				"TagEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					assert.True(t, !tagProcess(t, nonexistentID, "foo").Successful())
				},
				"TagEmptyTagFails": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					input, err := json.Marshal(TagIDInput{ID: jasperProcID})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, processTag(), []string{string(input)}, &OutcomeResponse{}))
				},
				"GetTags": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					tag := "foo"
					assert.True(t, tagProcess(t, jasperProcID, tag).Successful())

					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &TagsResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processGetTags(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.Tags), 1)
					assert.Equal(t, tag, resp.Tags[0])
				},
				"ResetTags": func(ctx context.Context, t *testing.T, c *cli.Command, jasperProcID string) {
					tag := "foo"
					assert.True(t, tagProcess(t, jasperProcID, tag).Successful())

					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processResetTags(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())

					getTagsInput, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					getTagsResp := &TagsResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, processGetTags(), []string{string(getTagsInput)}, getTagsResp))
					assert.True(t, getTagsResp.Successful())
					assert.Equal(t, len(getTagsResp.Tags), 0)
				},
			} {
				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()
					port := testutil.GetPortNumber()
					c := mockCLIContext(remoteType, port)
					manager := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))
					closeService := makeService(ctx, t, port, manager)
					defer func() {
						check.NotError(t, closeService())
					}()

					resp := &InfoResponse{}
					input, err := json.Marshal(testutil.SleepCreateOpts(1))
					assert.NotError(t, err)
					assert.NotError(t, execCLICommandInputOutput(t, managerCreateProcess(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					assert.NotZero(t, resp.Info.ID)

					testCase(ctx, t, c, resp.Info.ID)
				})
			}
		})
	}
}
