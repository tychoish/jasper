package splunk

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/splunk"
	"github.com/tychoish/jasper/options"
)

type LoggerConfig struct {
	Registry options.LoggerRegistry

	info     loggerConfigInfo
	producer options.LoggerProducer
	sender   send.Sender
}

type loggerConfigInfo struct {
	Type   string                        `json:"type" bson:"type"`
	Format options.RawLoggerConfigFormat `json:"format" bson:"format"`
	Config options.RawLoggerConfig       `json:"config" bson:"config"`
}

func TestLogger(t *testing.T) {
	t.Skip("fix concept")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for name, test := range map[string]func(t *testing.T, opts *options.Create){
		"ResolveFailsWithMismatchingLoggerConfiguration": func(t *testing.T, opts *options.Create) {
			config, err := json.Marshal(&LoggerOptions{
				Splunk: splunk.ConnectionInfo{
					ServerURL: "https://example.com/",
				},
			})
			fmt.Println(config)
			assert.NotError(t, err)
			// opts.Output.Loggers = []*LoggerConfig{
			// 	{
			// 		info: loggerConfigInfo{
			// 			Type:   LogType,
			// 			Format: options.RawLoggerConfigFormatJSON,
			// 			Config: config,
			// 		},
			// 	},
			// }
			cmd, _, err := opts.Resolve(ctx)
			check.Error(t, err)
			check.True(t, cmd == nil)
		},
		"ResolveFailsWithInvalidErrorLoggingConfiguration": func(t *testing.T, opts *options.Create) {
			config, err := json.Marshal(&LoggerOptions{
				Splunk: splunk.ConnectionInfo{
					ServerURL: "https://example.com/",
				},
			})
			fmt.Println(config)
			assert.NotError(t, err)
			// opts.Output.Loggers = []*options.LoggerConfig{
			// 	{
			// 		info: loggerConfigInfo{
			// 			Type:   LogType,
			// 			Format: options.RawLoggerConfigFormatJSON,
			// 			Config: config,
			// 		},
			// 	},
			// }
			opts.Output.SuppressOutput = true
			cmd, _, err := opts.Resolve(ctx)
			check.Error(t, err)
			check.True(t, cmd == nil)
		},
	} {
		t.Run(name, func(t *testing.T) {
			opts := &options.Create{Args: []string{"ls"}}
			test(t, opts)
		})

	}
}
