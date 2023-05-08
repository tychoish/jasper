package cli

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	"github.com/urfave/cli/v2"
)

func TestCLILoggingCache(t *testing.T) {
	for remoteType, makeService := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) util.CloseFunc{
		RESTService: makeTestRESTService,
		RPCService:  makeTestRPCService,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Context){
				"CreateSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context) {
					id := "id"
					logger := createCachedLoggerFromCLI(t, c, id)
					assert.Equal(t, id, logger.ID)
					assert.NotZero(t, logger.Accessed)
				},
				"CreateWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context) {
					logger, err := jasper.NewInMemoryLogger(100)
					assert.NotError(t, err)
					input, err := json.Marshal(LoggingCacheCreateInput{
						Output: options.Output{
							Loggers: []*options.LoggerConfig{logger},
						},
					})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.Error(t, execCLICommandInputOutput(t, c, loggingCacheCreate(), input, resp))
					assert.True(t, !resp.Successful())
					assert.Zero(t, resp.Logger)
				},
				"GetSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context) {
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheGet(), input, resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, logger.ID, resp.Logger.ID)
					assert.NotZero(t, resp.Logger.Accessed)
				},
				"GetWithEmptyIDFails": func(ctx context.Context, t *testing.T, c *cli.Context) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.Error(t, execCLICommandInputOutput(t, c, loggingCacheGet(), input, resp))
					assert.True(t, !resp.Successful())
				},
				"GetWithNonexistentIDFails": func(ctx context.Context, t *testing.T, c *cli.Context) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: "foo"})
					assert.NotError(t, err)
					resp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheGet(), input, resp))
					assert.True(t, !resp.Successful())
				},
				"RemoveSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context) {
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheRemove(), input, resp))
					assert.True(t, resp.Successful())

					getResp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheGet(), input, getResp))
					assert.True(t, getResp.IsZero())
					assert.True(t, !getResp.Successful())
				},
				"CloseAndRemoveSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context) {
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheCloseAndRemove(), input, resp))
					assert.True(t, resp.Successful())

					getResp := &CachedLoggerResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheGet(), input, getResp))
					assert.True(t, !getResp.Successful())
					assert.True(t, getResp.IsZero())
				},
				"ClearSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context) {
					_ = createCachedLoggerFromCLI(t, c, "id0")
					_ = createCachedLoggerFromCLI(t, c, "id1")

					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, loggingCacheClear(), resp))
					assert.True(t, resp.Successful())

					getResp := &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, loggingCacheLen(), getResp))
					check.True(t, getResp.Successful())
					assert.Zero(t, getResp.Length)
				},
				"RemoveWithNonexistentIDNoops": func(ctx context.Context, t *testing.T, c *cli.Context) {
					logger := createCachedLoggerFromCLI(t, c, "id")

					input, err := json.Marshal(IDInput{ID: "foo"})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheRemove(), input, resp))
					check.True(t, resp.Successful())

					getResp := &CachedLoggerResponse{}
					getInput, err := json.Marshal(IDInput{ID: logger.ID})
					assert.NotError(t, err)
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheGet(), getInput, getResp))
					check.True(t, getResp.Successful())
					assert.NotZero(t, getResp.Logger)
				},
				"LenSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					resp := &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, loggingCacheLen(), resp))
					assert.True(t, resp.Successful())
					assert.Equal(t, 1, resp.Length)
				},
				"PruneSucceeds": func(ctx context.Context, t *testing.T, c *cli.Context) {
					_ = createCachedLoggerFromCLI(t, c, "id")

					lenResp := &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, loggingCacheLen(), lenResp))
					assert.True(t, lenResp.Successful())
					assert.Equal(t, 1, lenResp.Length)

					input, err := json.Marshal(LoggingCachePruneInput{LastAccessed: time.Now().Add(time.Hour)})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, loggingCachePrune(), input, resp))
					check.True(t, resp.Successful())

					lenResp = &LoggingCacheLenResponse{}
					assert.NotError(t, execCLICommandOutput(t, c, loggingCacheLen(), lenResp))
					assert.True(t, lenResp.Successful())
					assert.Zero(t, lenResp.Length)
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
func createCachedLoggerFromCLI(t *testing.T, c *cli.Context, id string) options.CachedLogger {
	input, err := json.Marshal(LoggingCacheCreateInput{
		ID:     id,
		Output: validLoggingCacheOptions(t),
	})
	assert.NotError(t, err)
	resp := &CachedLoggerResponse{}
	assert.NotError(t, execCLICommandInputOutput(t, c, loggingCacheCreate(), input, resp))
	assert.True(t, resp.Successful())
	assert.Equal(t, id, resp.Logger.ID)

	return resp.Logger
}
