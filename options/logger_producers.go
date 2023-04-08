package options

import (
	"fmt"

	"github.com/tychoish/birch"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/splunk"
)

///////////////////////////////////////////////////////////////////////////////
// Default Logger
///////////////////////////////////////////////////////////////////////////////

// LogDefault is the type name for the default logger.
const LogDefault = "default"

// DefaultLoggerOptions packages the options for creating a default logger.
type DefaultLoggerOptions struct {
	Prefix string      `json:"prefix" bson:"prefix"`
	Base   BaseOptions `json:"base" bson:"base"`
}

// NewDefaultLoggerProducer returns a LoggerProducer backed by
// DefaultLoggerOptions.
func NewDefaultLoggerProducer() LoggerProducer { return &DefaultLoggerOptions{} }

// Validate ensures DefaultLoggerOptions is valid.
func (opts *DefaultLoggerOptions) Validate() error {
	if opts.Prefix == "" {
		opts.Prefix = DefaultLogName
	}

	return opts.Base.Validate()
}

func (*DefaultLoggerOptions) Type() string { return LogDefault }
func (opts *DefaultLoggerOptions) Configure() (send.Sender, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	sender := send.MakePlain()

	// TODO fix logger so that you can prefix the logger with the
	// name somehow
	sender.SetName(opts.Prefix)

	var err error
	sender, err = NewSafeSender(sender, opts.Base)
	if err != nil {
		return nil, fmt.Errorf("problem creating safe default logger: %w", err)
	}
	return sender, nil
}
func (opts *DefaultLoggerOptions) MarshalDocument() (*birch.Document, error) {
	base, err := opts.Base.MarshalDocument()
	if err != nil {
		return nil, err
	}

	return birch.DC.Elements(
		birch.EC.String("prefix", opts.Prefix),
		birch.EC.SubDocument("base", base),
	), nil
}

func (opts *DefaultLoggerOptions) MarshalBSON() ([]byte, error) {
	return birch.MarshalDocumentBSON(opts)
}

///////////////////////////////////////////////////////////////////////////////
// File Logger
///////////////////////////////////////////////////////////////////////////////

// LogFile is the type name for the file logger.
const LogFile = "file"

// FileLoggerOptions packages the options for creating a file logger.
type FileLoggerOptions struct {
	Filename string      `json:"filename " bson:"filename"`
	Base     BaseOptions `json:"base" bson:"base"`
}

// NewFileLoggerProducer returns a LoggerProducer backed by FileLoggerOptions.
func NewFileLoggerProducer() LoggerProducer { return &FileLoggerOptions{} }

// Validate ensures FileLoggerOptions is valid.
func (opts *FileLoggerOptions) Validate() error {
	catcher := &erc.Collector{}

	erc.When(catcher, opts.Filename == "", "must specify a filename")
	catcher.Add(opts.Base.Validate())
	return catcher.Resolve()
}

func (*FileLoggerOptions) Type() string { return LogFile }
func (opts *FileLoggerOptions) Configure() (send.Sender, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	sender, err := send.MakePlainFile(opts.Filename)
	if err != nil {
		return nil, fmt.Errorf("problem creating base file logger: %w", err)
	}

	sender.SetName(DefaultLogName)

	sender, err = NewSafeSender(sender, opts.Base)
	if err != nil {
		return nil, fmt.Errorf("problem creating safe file logger: %w", err)
	}
	return sender, nil
}

///////////////////////////////////////////////////////////////////////////////
// Inherited Logger
///////////////////////////////////////////////////////////////////////////////

// LogInherited is the type name for the inherited logger.
const LogInherited = "inherited"

// InheritLoggerOptions packages the options for creating an inherited logger.
type InheritedLoggerOptions struct {
	Base BaseOptions `json:"base" bson:"base"`
}

// NewInheritedLoggerProducer returns a LoggerProducer backed by
// InheritedLoggerOptions.
func NewInheritedLoggerProducer() LoggerProducer { return &InheritedLoggerOptions{} }

func (*InheritedLoggerOptions) Type() string { return LogInherited }
func (opts *InheritedLoggerOptions) Configure() (send.Sender, error) {
	var (
		sender send.Sender
		err    error
	)

	if err = opts.Base.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	sender = grip.Sender()
	sender.SetPriority(opts.Base.Level)

	sender, err = NewSafeSender(sender, opts.Base)
	if err != nil {
		return nil, fmt.Errorf("problem creating safe inherited logger: %w", err)
	}
	return sender, nil
}

///////////////////////////////////////////////////////////////////////////////
// In Memory Logger
///////////////////////////////////////////////////////////////////////////////

// LogInMemory is the type name for the in memory logger.
const LogInMemory = "in-memory"

// InMemoryLoggerOptions packages the options for creating an in memory logger.
type InMemoryLoggerOptions struct {
	InMemoryCap int         `json:"in_memory_cap" bson:"in_memory_cap"`
	Base        BaseOptions `json:"base" bson:"base"`
}

// NewInMemoryLoggerProducer returns a LoggerProducer backed by
// InMemoryLoggerOptions.
func NewInMemoryLoggerProducer() LoggerProducer { return &InMemoryLoggerOptions{} }

func (opts *InMemoryLoggerOptions) Validate() error {
	catcher := &erc.Collector{}

	erc.When(catcher, opts.InMemoryCap <= 0, "invalid in-memory capacity")
	catcher.Add(opts.Base.Validate())
	return catcher.Resolve()
}

func (*InMemoryLoggerOptions) Type() string { return LogInMemory }
func (opts *InMemoryLoggerOptions) Configure() (send.Sender, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	sender, err := send.NewInMemorySender(DefaultLogName, opts.Base.Level, opts.InMemoryCap)
	if err != nil {
		return nil, fmt.Errorf("problem creating base in-memory logger: %w", err)
	}

	sender, err = NewSafeSender(sender, opts.Base)
	if err != nil {
		return nil, fmt.Errorf("problem creating safe in-memory logger: %w", err)
	}
	return sender, nil
}

///////////////////////////////////////////////////////////////////////////////
// Splunk Logger
///////////////////////////////////////////////////////////////////////////////

// LogSplunk is the type name for the splunk logger.
const LogSplunk = "splunk"

// SplunkLoggerOptions packages the options for creating a splunk logger.
type SplunkLoggerOptions struct {
	Splunk splunk.ConnectionInfo `json:"splunk" bson:"splunk"`
	Base   BaseOptions           `json:"base" bson:"base"`
}

// SplunkLoggerProducer returns a LoggerProducer backed by SplunkLoggerOptions.
func NewSplunkLoggerProducer() LoggerProducer { return &SplunkLoggerOptions{} }

func (opts *SplunkLoggerOptions) Validate() error {
	catcher := &erc.Collector{}

	erc.When(catcher, opts.Splunk.Populated(), "missing connection info for output type splunk")
	catcher.Add(opts.Base.Validate())
	return catcher.Resolve()
}

func (*SplunkLoggerOptions) Type() string { return LogSplunk }
func (opts *SplunkLoggerOptions) Configure() (send.Sender, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	sender, err := splunk.MakeSender(opts.Splunk)
	if err != nil {
		return nil, fmt.Errorf("problem creating base splunk logger: %w", err)
	}
	sender.SetName(DefaultLogName)
	sender.SetPriority(opts.Base.Level)
	sender, err = NewSafeSender(sender, opts.Base)
	if err != nil {
		return nil, fmt.Errorf("problem creating safe splunk logger: %w", err)
	}
	return sender, nil
}
