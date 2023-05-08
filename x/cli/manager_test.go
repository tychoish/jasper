package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/mock"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	"github.com/urfave/cli/v2"
)

func TestCLIManager(t *testing.T) {
	for remoteType, makeService := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) util.CloseFunc{
		RESTService: makeTestRESTService,
		RPCService:  makeTestRPCService,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string){
				"IDReturnsNonempty": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					resp := &IDResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, managerID(), resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.ID), 0)
				},
				"CommandsWithInputFailWithInvalidInput": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(mock.Process{})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, managerCreateProcess(), input, &InfoResponse{}))
					assert.Error(t, execCLICommandInputOutput(t, c, managerCreateCommand(), input, &OutcomeResponse{}))
					assert.Error(t, execCLICommandInputOutput(t, c, managerGet(), input, &InfoResponse{}))
					assert.Error(t, execCLICommandInputOutput(t, c, managerList(), input, &InfosResponse{}))
					assert.Error(t, execCLICommandInputOutput(t, c, managerGroup(), input, &InfosResponse{}))
				},
				"CommandsWithoutInputPassWithInvalidInput": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(mock.Process{})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					check.NotError(t, execCLICommandInputOutput(t, c, managerClear(), input, resp))
					check.NotError(t, execCLICommandInputOutput(t, c, managerClose(), input, resp))
				},
				"CreateProcessPasses": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(options.Create{
						Args: []string{"echo", "hello", "world"},
					})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, managerCreateProcess(), input, resp))
					assert.True(t, resp.Successful())
				},
				"CreateCommandPasses": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(options.Command{
						Commands: [][]string{{"true"}},
					})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, managerCreateCommand(), input, resp))
					assert.True(t, resp.Successful())
				},
				"GetExistingIDPasses": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{jasperProcID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, managerGet(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, jasperProcID, resp.Info.ID)
				},
				"GetNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{nonexistentID})
					assert.NotError(t, err)
					resp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, managerGet(), input, resp))
					assert.True(t, !resp.Successful())
					assert.NotEqual(t, len(resp.ErrorMessage()), 0)
				},
				"GetEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(IDInput{""})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, managerGet(), input, &InfoResponse{}))
				},
				"ListValidFilterPasses": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(FilterInput{options.All})
					assert.NotError(t, err)
					resp := &InfosResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, managerList(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.Infos), 1)
					assert.Equal(t, jasperProcID, resp.Infos[0].ID)
				},
				"ListInvalidFilterFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(FilterInput{options.Filter("foo")})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, managerList(), input, &InfosResponse{}))
				},
				"GroupFindsTaggedProcess": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					tag := "foo"
					assert.True(t, tagProcess(t, c, jasperProcID, tag).Successful())

					input, err := json.Marshal(TagInput{Tag: tag})
					assert.NotError(t, err)
					resp := &InfosResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, managerGroup(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.Infos), 1)
					assert.Equal(t, jasperProcID, resp.Infos[0].ID)
				},
				"GroupEmptyTagFails": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(TagInput{Tag: ""})
					assert.NotError(t, err)
					assert.Error(t, execCLICommandInputOutput(t, c, managerGroup(), input, &InfosResponse{}))
				},
				"GroupNoMatchingTaggedProcessesReturnsEmpty": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					input, err := json.Marshal(TagInput{Tag: "foo"})
					assert.NotError(t, err)
					resp := &InfosResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, managerGroup(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, len(resp.Infos), 0)
				},
				"ClearPasses": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, managerClear(), resp))
					check.True(t, resp.Successful())
				},
				"ClosePasses": func(ctx context.Context, t *testing.T, c *cli.Context, jasperProcID string) {
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, managerClose(), resp))
					check.True(t, resp.Successful())
				},
			} {
				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()
					port := testutil.GetPortNumber()
					c := mockCLIContext(remoteType, port)
					manager := jasper.NewManager(jasper.ManagerOptions{Synchronized: true})
					closeService := makeService(ctx, t, port, manager)
					defer func() {
						check.NotError(t, closeService())
					}()

					resp := &InfoResponse{}
					input, err := json.Marshal(testutil.TrueCreateOpts())
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
