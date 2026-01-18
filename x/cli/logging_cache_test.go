package cli

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	"github.com/urfave/cli/v3"
)

func TestCLILoggingCache(t *testing.T) {
	for remoteType, makeService := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) util.CloseFunc{
		RESTService: makeTestRESTService,
		RPCService:  makeTestRPCService,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Command){
				"CreateSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					id := "id"
					logger := createCachedLoggerFromCLI(t, c, id)
					assert.Equal(t, id, logger.ID)
					assert.NotZero(t, logger.Accessed)
				},
				"CreateWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Command) {
					logger, err := jasper.NewInMemoryLogger(100)
					assert.NotError(t, err)
					input, err := json.Marshal(LoggingCacheCreateInput{
						Output: options.Output{
							Loggers: []*options.LoggerConfig{logger},
						},
					})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.Error(t, execCLICommandInputOutput(t, loggingCacheCreate(), []string{string(input)}, resp))
					assert.True(t, !resp.Successful())
					assert.Zero(t, resp.Logger)
				},
				"GetSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheGet(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, logger.ID, resp.Logger.ID)
					assert.NotZero(t, resp.Logger.Accessed)
				},
				"GetWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Command) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.Error(t, execCLICommandInputOutput(t, loggingCacheGet(), []string{string(input)}, resp))
					assert.True(t, !resp.Successful())
				},
				"GetWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Command) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: "foo"})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheGet(), []string{string(input)}, resp))
					assert.True(t, !resp.Successful())
				},
				"RemoveSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					t.Skip("remove is a problem")
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheRemove(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())

					getResp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheGet(), []string{string(input)}, getResp))
					testt.Log(t, getResp.Message)
					assert.True(t, getResp.IsZero())
					assert.True(t, !getResp.Successful())
				},
				"CloseAndRemoveSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					t.Skip("remove unclear")
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheCloseAndRemove(), []string{string(input)}, resp))
					assert.True(t, resp.Successful())

					getResp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheGet(), []string{string(input)}, getResp))
					testt.Log(t, getResp.Message)
					assert.True(t, !getResp.Successful())
					assert.True(t, getResp.IsZero())
				},
				"ClearSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					_ = createCachedLoggerFromCLI(t, c, "id0")
					_ = createCachedLoggerFromCLI(t, c, "id1")

					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandOutput(t, loggingCacheClear(), nil, resp))
					assert.True(t, resp.Successful())

					getResp := &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, loggingCacheLen(), nil, getResp))
					check.True(t, getResp.Successful())
					assert.Zero(t, getResp.Length)
				},
				"RemoveWithNonexistentIDNoops": func(ctx context.Context, t *testing.T, c *cli.Command) {
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: "foo"})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheRemove(), []string{string(input)}, resp))
					check.True(t, resp.Successful())

					getResp := &CachedLoggerResponse{}
					getInput, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					assert.NotError(t, execCLICommandInputOutput(t, loggingCacheGet(), []string{string(getInput)}, getResp))
					check.True(t, getResp.Successful())
					assert.NotZero(t, getResp.Logger)
				},
				"LenSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					resp := &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, loggingCacheLen(), nil, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, 1, resp.Length)
				},
				"PruneSucceeds": func(ctx context.Context, t *testing.T, c *cli.Command) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					lenResp := &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, loggingCacheLen(), nil, lenResp))
					assert.True(t, lenResp.Successful())
					assert.Equal(t, 1, lenResp.Length)

					input, err := json.Marshal(LoggingCachePruneInput{LastAccessed: time.Now().Add(time.Hour)})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, loggingCachePrune(), []string{string(input)}, resp))
					check.True(t, resp.Successful())

					lenResp = &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, loggingCacheLen(), []string{string(input)}, lenResp))
					assert.True(t, lenResp.Successful())
					assert.Zero(t, lenResp.Length)
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

// validLoggingCacheOptions returns valid options for creating a cached logger.
func validLoggingCacheOptions(t *testing.T) options.Output {
	logger, err := jasper.NewInMemoryLogger(100)
	assert.NotError(t, err)
	return options.Output{
		Loggers: []*options.LoggerConfig{logger},
	}
}

// createCachedLoggerFromCLI creates a cached logger on a remote service using
// the CLI.
func createCachedLoggerFromCLI(t *testing.T, c *cli.Command, id string) options.CachedLogger {
	input, err := json.Marshal(LoggingCacheCreateInput{
		ID:     id,
		Output: validLoggingCacheOptions(t),
	})
	assert.NotError(t, err)
	resp := &CachedLoggerResponse{}
	assert.NotError(t, execCLICommandInputOutput(t, loggingCacheCreate(), []string{string(input)}, resp))
	testt.Log(t, resp)
	assert.True(t, resp.Successful())
	assert.Equal(t, id, resp.Logger.ID)

	return resp.Logger
}
