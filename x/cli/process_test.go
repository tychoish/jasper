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
	"github.com/urfave/cli"
)

func tagProcess(t *testing.T, c *cli.Context, jasperProcID string, tag string) OutcomeResponse {
	input, err := json.Marshal(TagIDInput{ID: jasperProcID, Tag: tag})
	assert.NotError(t, err)
	resp := OutcomeResponse{}
	assert.NotError(t, execCLICommandInputOutput(t, c, processTag(), input, &resp))
	return resp
}

const nonexistentID = "nonexistent"

func TestCLIProcess(t *testing.T) {
	for remoteType, makeService := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) util.CloseFunc{
		RESTService: makeTestRESTService,
		RPCService:  makeTestRPCService,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string){
				"InfoWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processInfo(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, jasperProcID, resp.Info.ID)
					check.True(t, resp.Info.IsRunning)
				},
				"InfoWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processInfo(), input, resp))
					assert.True(t, !resp.Successful())
				},
				"InfoWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, processInfo(), input, &InfoResponse{}))
				},
				"RunningWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &RunningResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processRunning(), input, resp))
					assert.True(t, resp.Successful())
					check.True(t, resp.Running)
				},
				"RunningWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &RunningResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processRunning(), input, resp))
					assert.True(t, !resp.Successful())
				},
				"RunningWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, processRunning(), input, &RunningResponse{}))
				},
				"CompleteWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &CompleteResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processComplete(), input, resp))
					assert.True(t, resp.Successful())
					assert.True(t, !resp.Complete)
				},
				"CompleteWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &CompleteResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processComplete(), input, resp))
					assert.True(t, !resp.Successful())
				},
				"CompleteWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, processComplete(), input, &CompleteResponse{}))
				},
				"WaitWithExistingIDSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &WaitResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processWait(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.Error), 0)
					assert.Zero(t, resp.ExitCode)
				},
				"WaitWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &WaitResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processWait(), input, resp))
					assert.True(t, !resp.Successful())
				},
				"WaitWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, processWait(), input, &WaitResponse{}))
				},
				"Respawn": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processRespawn(), input, resp))
					assert.True(t, resp.Successful())
					assert.NotZero(t, resp.Info.ID)
					assert.NotEqual(t, resp.Info.ID, jasperProcID)
				},
				"RegisterSignalTriggerID": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(SignalTriggerIDInput{ID: jasperProcID, SignalTriggerID: jasper.CleanTerminationSignalTrigger})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processRegisterSignalTriggerID(), input, resp))
					assert.True(t, resp.Successful())
				},
				"Tag": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					assert.True(t, tagProcess(t, c, jasperProcID, "foo").Successful())
				},
				"TagNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					assert.True(t, !tagProcess(t, c, nonexistentID, "foo").Successful())
				},
				"TagEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					assert.True(t, !tagProcess(t, c, nonexistentID, "foo").Successful())
				},
				"TagEmptyTagFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(TagIDInput{ID: jasperProcID})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, processTag(), input, &OutcomeResponse{}))
				},
				"GetTags": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					tag := "foo"
					assert.True(t, tagProcess(t, c, jasperProcID, tag).Successful())

					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &TagsResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processGetTags(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.Tags), 1)
					assert.Equal(t, tag, resp.Tags[0])
				},
				"ResetTags": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					tag := "foo"
					assert.True(t, tagProcess(t, c, jasperProcID, tag).Successful())

					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processResetTags(), input, resp))
					assert.True(t, resp.Successful())

					getTagsInput, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					getTagsResp := &TagsResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, processGetTags(), getTagsInput, getTagsResp))
					assert.True(t, getTagsResp.Successful())
					assert.Equal(t, len(getTagsResp.Tags), 0)
				},
			} {
				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()
					port := testutil.GetPortNumber()
					c := mockCLIContext(remoteType, port)
					manager, err := jasper.NewSynchronizedManager(false)
					assert.NotError(t, err)
					closeService := makeService(ctx, t, port, manager)
					assert.NotError(t, err)
					defer func() {
						check.NotError(t, closeService())
					}()

					resp := &InfoResponse{}
					input, err := json.Marshal(testutil.SleepCreateOpts(1))
					assert.NotError(t, err)
					assert.NotError(t, execCLICommandInputOutput(t, c, managerCreateProcess(), input, resp))
					assert.True(t, resp.Successful())
					assert.NotZero(t, resp.Info.ID)

					testCase(ctx, t, c, resp.Info.ID)
				})
			}
		})
	}
}
