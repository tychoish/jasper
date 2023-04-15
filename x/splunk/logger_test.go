package splunk

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			config, err := json.Marshal(&SplunkLoggerOptions{
				Splunk: splunk.ConnectionInfo{
					ServerURL: "https://example.com/",
				},
			})
			fmt.Println(config)
			require.NoError(t, err)
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
			assert.Error(t, err)
			assert.Nil(t, cmd)
		},
		"ResolveFailsWithInvalidErrorLoggingConfiguration": func(t *testing.T, opts *options.Create) {
			config, err := json.Marshal(&SplunkLoggerOptions{
				Splunk: splunk.ConnectionInfo{
					ServerURL: "https://example.com/",
				},
			})
			fmt.Println(config)
			require.NoError(t, err)
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
			assert.Error(t, err)
			assert.Nil(t, cmd)
		},
	} {
		t.Run(name, func(t *testing.T) {
			opts := &options.Create{Args: []string{"ls"}}
			test(t, opts)
		})

	}
}
