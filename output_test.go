package jasper

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func TestGetInMemoryLogStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for procType, makeProc := range map[string]ProcessConstructor{
		"Basic":    NewBasicProcess,
		"Blocking": NewBlockingProcess,
	} {
		t.Run(procType, func(t *testing.T) {

			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, opts *options.Create, makeProc ProcessConstructor, output string){
				"FailsWithNilProcess": func(ctx context.Context, t *testing.T, opts *options.Create, makeProc ProcessConstructor, output string) {
					logs, err := GetInMemoryLogStream(ctx, nil, 1)
					check.Error(t, err)
					check.Nil(t, logs)
				},
				"FailsWithInvalidCount": func(ctx context.Context, t *testing.T, opts *options.Create, makeProc ProcessConstructor, output string) {
					proc, err := makeProc(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					logs, err := GetInMemoryLogStream(ctx, proc, 0)
					check.Error(t, err)
					check.Nil(t, logs)
				},
				"FailsWithoutInMemoryLogger": func(ctx context.Context, t *testing.T, opts *options.Create, makeProc ProcessConstructor, output string) {
					proc, err := makeProc(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					logs, err := GetInMemoryLogStream(ctx, proc, 100)
					check.Error(t, err)
					check.Nil(t, logs)
				},
				"SucceedsWithInMemoryLogger": func(ctx context.Context, t *testing.T, opts *options.Create, makeProc ProcessConstructor, output string) {
					loggerProducer := &options.InMemoryLoggerOptions{
						InMemoryCap: 100,
						Base: options.BaseOptions{
							Format: options.LogFormatPlain,
						},
					}
					config := &options.LoggerConfig{}
					assert.NotError(t, config.Set(loggerProducer))
					opts.Output.Loggers = []*options.LoggerConfig{config}
					proc, err := makeProc(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					logs, err := GetInMemoryLogStream(ctx, proc, 100)
					check.NotError(t, err)
					check.Contains(t, logs, output)
				},
				"MultipleInMemoryLoggersReturnLogsFromOnlyOne": func(ctx context.Context, t *testing.T, opts *options.Create, makeProc ProcessConstructor, output string) {
					config1 := &options.LoggerConfig{}
					assert.NotError(t, config1.Set(&options.InMemoryLoggerOptions{
						InMemoryCap: 100,
						Base: options.BaseOptions{
							Format: options.LogFormatPlain,
						},
					}))
					config2 := &options.LoggerConfig{}
					assert.NotError(t, config2.Set(&options.InMemoryLoggerOptions{
						InMemoryCap: 100,
						Base: options.BaseOptions{
							Format: options.LogFormatPlain,
						},
					}))
					opts.Output.Loggers = []*options.LoggerConfig{config1, config2}
					proc, err := makeProc(ctx, opts)
					assert.NotError(t, err)

					_, err = proc.Wait(ctx)
					assert.NotError(t, err)

					logs, err := GetInMemoryLogStream(ctx, proc, 100)
					check.NotError(t, err)
					check.Contains(t, logs, output)

					outputCount := 0
					for _, log := range logs {
						if strings.Contains(log, output) {
							outputCount++
						}
					}
					check.Equal(t, 1, outputCount)
				},
				// "": func(ctx context.Context, t *testing.T, opts *options.Create, output string) {},
			} {
				t.Run(testName, func(t *testing.T) {
					tctx, tcancel := context.WithTimeout(ctx, testutil.ProcessTestTimeout)
					defer tcancel()

					output := t.Name() + " " + filepath.Join(procType, testName)
					opts := &options.Create{Args: []string{"echo", output}}
					testCase(tctx, t, opts, makeProc, output)
				})
			}

		})
	}
}
