package splunk

import (
	"fmt"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/splunk"
	"github.com/tychoish/jasper/options"
)

///////////////////////////////////////////////////////////////////////////////
// Splunk Logger
///////////////////////////////////////////////////////////////////////////////

func init() {
	reg := options.GetGlobalLoggerRegistry()
	reg.Register(NewSplunkLoggerProducer)
}

// LogType is the type name for the splunk logger.
const LogType = "splunk"

// SplunkLoggerOptions packages the options for creating a splunk logger.
type SplunkLoggerOptions struct {
	Splunk splunk.ConnectionInfo `json:"splunk" bson:"splunk"`
	Base   options.BaseOptions   `json:"base" bson:"base"`
}

// SplunkLoggerProducer returns a LoggerProducer backed by SplunkLoggerOptions.
func NewSplunkLoggerProducer() options.LoggerProducer { return &SplunkLoggerOptions{} }

func (opts *SplunkLoggerOptions) Validate() error {
	catcher := &erc.Collector{}

	erc.When(catcher, opts.Splunk.Populated(), "missing connection info for output type splunk")
	catcher.Add(opts.Base.Validate())
	return catcher.Resolve()
}

func (*SplunkLoggerOptions) Type() string { return LogType }
func (opts *SplunkLoggerOptions) Configure() (send.Sender, error) {
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
