package splunk

import (
	"fmt"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/splunk"
	"github.com/tychoish/jasper/options"
)

///////////////////////////////////////////////////////////////////////////////
// Splunk Logger
///////////////////////////////////////////////////////////////////////////////

func init() {
	reg := options.GetGlobalLoggerRegistry()
	reg.Register(NewLoggerProducer)
}

// LogType is the type name for the splunk logger.
const LogType = "splunk"

// LoggerOptions packages the options for creating a splunk logger.
type LoggerOptions struct {
	Splunk splunk.ConnectionInfo `json:"splunk" bson:"splunk"`
	Base   options.BaseOptions   `json:"base" bson:"base"`
}

// SplunkLoggerProducer returns a LoggerProducer backed by SplunkLoggerOptions.
func NewLoggerProducer() options.LoggerProducer { return &LoggerOptions{} }

func (opts *LoggerOptions) Validate() error {
	catcher := &erc.Collector{}

	catcher.If(opts.Splunk.Populated(), ers.Error("missing connection info for output type splunk"))
	catcher.Push(opts.Base.Validate())
	return catcher.Resolve()
}

func (*LoggerOptions) Type() string { return LogType }
func (opts *LoggerOptions) Configure() (send.Sender, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	sender, err := splunk.MakeSender(opts.Splunk)
	if err != nil {
		return nil, fmt.Errorf("problem creating base splunk logger: %w", err)
	}
	sender.SetName(options.DefaultLogName)
	sender.SetPriority(opts.Base.Level)
	sender, err = options.NewSafeSender(sender, opts.Base)
	if err != nil {
		return nil, fmt.Errorf("problem creating safe splunk logger: %w", err)
	}
	return sender, nil
}
