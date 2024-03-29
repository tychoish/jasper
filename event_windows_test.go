package jasper

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
)

const (
	mongodStartupTime         = 15 * time.Second
	mongodShutdownEventPrefix = "Global\\Mongo_"
)

func TestMongodShutdownEvent(t *testing.T) {
	for procName, makeProc := range map[string]ProcessConstructor{
		"Basic":    newBasicProcess,
		"Blocking": newBlockingProcess,
	} {
		t.Run(procName, func(t *testing.T) {
			if testing.Short() {
				t.Skip("skipping mongod shutdown event tests in short mode")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var opts options.Create
			dir, mongodPath := downloadMongoDB(t)
			defer os.RemoveAll(dir)

			optslist, dbPaths, err := setupMongods(1, mongodPath)
			assert.NotError(t, err)
			defer removeDBPaths(dbPaths)
			assert.Equal(t, 1, len(optslist))

			opts = optslist[0]
			logger := &options.LoggerConfig{}
			assert.NotError(t, logger.Set(&options.DefaultLoggerOptions{
				Base: options.BaseOptions{
					Format: options.LogFormatPlain,
				},
			}))
			opts.Output.Loggers = []*options.LoggerConfig{logger}

			proc, err := makeProc(ctx, &opts)
			assert.NotError(t, err)

			// Give mongod time to start up its signal processing thread.
			time.Sleep(mongodStartupTime)

			pid := proc.Info(ctx).PID
			mongodShutdownEvent := mongodShutdownEventPrefix + strconv.Itoa(pid)

			assert.NotError(t, SignalEvent(ctx, mongodShutdownEvent))

			exitCode, err := proc.Wait(ctx)
			check.NotError(t, err)
			check.Zero(t, exitCode)
			check.True(t, !proc.Running(ctx))
		})
	}
}
