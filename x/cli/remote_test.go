package cli

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	roptions "github.com/tychoish/jasper/x/remote/options"
	"github.com/urfave/cli/v3"
)

func TestCLIRemote(t *testing.T) {
	for remoteType, makeService := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) util.CloseFunc{
		RESTService: makeTestRESTService,
		RPCService:  makeTestRPCService,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Command){
				"DownloadFileSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					tmpFile, err := os.CreateTemp(testutil.BuildDirectory(), "out.txt")
					assert.NotError(t, err)
					defer func() {
						check.NotError(t, tmpFile.Close())
						check.NotError(t, os.RemoveAll(tmpFile.Name()))
					}()

					input, err := json.Marshal(roptions.Download{
						URL:  "https://example.com",
						Path: tmpFile.Name(),
					})
					assert.NotError(t, err)

					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, remoteDownloadFile(), []string{string(input)}, resp))

					_, err = os.Stat(tmpFile.Name())
					assert.NotError(t, err)
				},
				"GetLogStreamSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					inMemLogger, err := jasper.NewInMemoryLogger(10)
					assert.NotError(t, err)
					opts := testutil.TrueCreateOpts()
					opts.Output.Loggers = []*options.LoggerConfig{inMemLogger}
					createInput, err := json.Marshal(opts)
					assert.NotError(t, err)
					createResp := &InfoResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, managerCreateProcess(), []string{string(createInput)}, createResp))

					input, err := json.Marshal(LogStreamInput{ID: createResp.Info.ID, Count: 100})
					assert.NotError(t, err)
					resp := &LogStreamResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, remoteGetLogStream(), []string{string(input)}, resp))

					check.True(t, resp.Successful())
				},
				"WriteFileSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					tmpFile, err := os.CreateTemp(testutil.BuildDirectory(), "write_file")
					assert.NotError(t, err)
					defer func() {
						check.NotError(t, tmpFile.Close())
						check.NotError(t, os.RemoveAll(tmpFile.Name()))
					}()

					opts := options.WriteFile{Path: tmpFile.Name(), Content: []byte("foo")}
					input, err := json.Marshal(opts)
					assert.NotError(t, err)
					resp := &OutcomeResponse{}

					assert.NotError(t, execCLICommandInputOutput(t, remoteWriteFile(), []string{string(input)}, resp))

					check.True(t, resp.Successful())

					data, err := os.ReadFile(opts.Path)
					assert.NotError(t, err)
					assert.Equal(t, string(opts.Content), string(data))
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

					testCase(ctx, t, c)
				})
			}
		})
	}
}
