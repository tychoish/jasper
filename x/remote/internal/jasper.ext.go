package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/x/splunk"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
	roptions "github.com/tychoish/jasper/x/remote/options"
	jsplunk "github.com/tychoish/jasper/x/splunk"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Export takes a protobuf RPC CreateOptions struct and returns the analogous
// Jasper CreateOptions struct. It is not safe to concurrently access the
// exported RPC CreateOptions and the returned Jasper CreateOptions.
func (opts *CreateOptions) Export() (*options.Create, error) {
	out := &options.Create{
		Args:               opts.Args,
		Environment:        opts.Environment,
		WorkingDirectory:   opts.WorkingDirectory,
		Timeout:            time.Duration(opts.TimeoutSeconds) * time.Second,
		TimeoutSecs:        int(opts.TimeoutSeconds),
		OverrideEnviron:    opts.OverrideEnviron,
		Tags:               opts.Tags,
		StandardInputBytes: opts.StandardInputBytes,
	}
	if len(opts.StandardInputBytes) != 0 {
		out.StandardInput = bytes.NewBuffer(opts.StandardInputBytes)
	}

	if opts.Output != nil {
		exportedOutput, err := opts.Output.Export()
		if err != nil {
			return nil, fmt.Errorf("problem exporting output: %w", err)
		}
		out.Output = exportedOutput
	}

	for _, opt := range opts.OnSuccess {
		exportedOpt, err := opt.Export()
		if err != nil {
			return nil, fmt.Errorf("problem exporting create options: %w", err)
		}
		out.OnSuccess = append(out.OnSuccess, exportedOpt)
	}
	for _, opt := range opts.OnFailure {
		exportedOpt, err := opt.Export()
		if err != nil {
			return nil, fmt.Errorf("problem exporting create options: %w", err)
		}
		out.OnFailure = append(out.OnFailure, exportedOpt)
	}
	for _, opt := range opts.OnTimeout {
		exportedOpt, err := opt.Export()
		if err != nil {
			return nil, fmt.Errorf("problem exporting create options: %w", err)
		}
		out.OnTimeout = append(out.OnTimeout, exportedOpt)
	}

	return out, nil
}

// ConvertCreateOptions takes a Jasper CreateOptions struct and returns an
// equivalent protobuf RPC *CreateOptions struct. ConvertCreateOptions is the
// inverse of (*CreateOptions) Export(). It is not safe to concurrently
// access the converted Jasper CreateOptions and the returned RPC
// CreateOptions.
func ConvertCreateOptions(opts *options.Create) (*CreateOptions, error) {
	if opts.TimeoutSecs == 0 && opts.Timeout != 0 {
		opts.TimeoutSecs = int(opts.Timeout.Seconds())
	}

	output, err := ConvertOutputOptions(opts.Output)
	if err != nil {
		return nil, fmt.Errorf("problem converting output options: %w", err)
	}

	co := &CreateOptions{
		Args:               opts.Args,
		Environment:        opts.Environment,
		WorkingDirectory:   opts.WorkingDirectory,
		TimeoutSeconds:     int64(opts.TimeoutSecs),
		OverrideEnviron:    opts.OverrideEnviron,
		Tags:               opts.Tags,
		Output:             &output,
		StandardInputBytes: opts.StandardInputBytes,
	}

	for _, opt := range opts.OnSuccess {
		convertedOpts, err := ConvertCreateOptions(opt)
		if err != nil {
			return nil, fmt.Errorf("problem converting create options: %w", err)
		}
		co.OnSuccess = append(co.OnSuccess, convertedOpts)
	}
	for _, opt := range opts.OnFailure {
		convertedOpts, err := ConvertCreateOptions(opt)
		if err != nil {
			return nil, fmt.Errorf("problem converting create options: %w", err)
		}
		co.OnFailure = append(co.OnFailure, convertedOpts)
	}
	for _, opt := range opts.OnTimeout {
		convertedOpts, err := ConvertCreateOptions(opt)
		if err != nil {
			return nil, fmt.Errorf("problem converting create options: %w", err)
		}
		co.OnTimeout = append(co.OnTimeout, convertedOpts)
	}

	return co, nil
}

// Export takes a protobuf RPC ProcessInfo struct and returns the analogous
// Jasper ProcessInfo struct.
func (info *ProcessInfo) Export() (jasper.ProcessInfo, error) {
	var startAt time.Time
	var err error
	if info.StartAt != nil {
		startAt = info.StartAt.AsTime()
	}
	var endAt time.Time
	if info.EndAt != nil {
		endAt = info.EndAt.AsTime()
	}
	opts, err := info.Options.Export()
	if err != nil {
		return jasper.ProcessInfo{}, fmt.Errorf("problem exporting create options: %w", err)
	}
	return jasper.ProcessInfo{
		ID:         info.Id,
		PID:        int(info.Pid),
		IsRunning:  info.Running,
		Successful: info.Successful,
		Complete:   info.Complete,
		ExitCode:   int(info.ExitCode),
		Timeout:    info.Timedout,
		Options:    *opts,
		StartAt:    startAt,
		EndAt:      endAt,
	}, nil
}

// ConvertProcessInfo takes a Jasper ProcessInfo struct and returns an
// equivalent protobuf RPC *ProcessInfo struct. ConvertProcessInfo is the
// inverse of (*ProcessInfo) Export().
func ConvertProcessInfo(info jasper.ProcessInfo) (*ProcessInfo, error) {
	opts, err := ConvertCreateOptions(&info.Options)
	if err != nil {
		return nil, fmt.Errorf("problem converting create options: %w", err)
	}
	return &ProcessInfo{
		Id:         info.ID,
		Pid:        int64(info.PID),
		ExitCode:   int32(info.ExitCode),
		Running:    info.IsRunning,
		Successful: info.Successful,
		Complete:   info.Complete,
		Timedout:   info.Timeout,
		StartAt:    timestamppb.New(info.StartAt),
		EndAt:      timestamppb.New(info.EndAt),
		Options:    opts,
	}, nil
}

// Export takes a protobuf RPC Signals struct and returns the analogous
// syscall.Signal.
func (s Signals) Export() syscall.Signal {
	switch s {
	case Signals_HANGUP:
		return syscall.SIGHUP
	case Signals_INIT:
		return syscall.SIGINT
	case Signals_TERMINATE:
		return syscall.SIGTERM
	case Signals_KILL:
		return syscall.SIGKILL
	case Signals_ABRT:
		return syscall.SIGABRT
	default:
		return syscall.Signal(0)
	}
}

// ConvertSignal takes a syscall.Signal and returns an
// equivalent protobuf RPC Signals struct. ConvertSignals is the
// inverse of (Signals) Export().
func ConvertSignal(s syscall.Signal) Signals {
	switch s {
	case syscall.SIGHUP:
		return Signals_HANGUP
	case syscall.SIGINT:
		return Signals_INIT
	case syscall.SIGTERM:
		return Signals_TERMINATE
	case syscall.SIGKILL:
		return Signals_KILL
	default:
		return Signals_UNKNOWN
	}
}

// ConvertFilter takes a Jasper Filter struct and returns an
// equivalent protobuf RPC *Filter struct.
func ConvertFilter(f options.Filter) *Filter {
	switch f {
	case options.All:
		return &Filter{Name: FilterSpecifications_ALL}
	case options.Running:
		return &Filter{Name: FilterSpecifications_RUNNING}
	case options.Terminated:
		return &Filter{Name: FilterSpecifications_TERMINATED}
	case options.Failed:
		return &Filter{Name: FilterSpecifications_FAILED}
	case options.Successful:
		return &Filter{Name: FilterSpecifications_SUCCESSFUL}
	default:
		return nil
	}
}

// Export takes a protobuf RPC OutputOptions struct and returns the analogous
// Jasper OutputOptions struct.
func (opts OutputOptions) Export() (options.Output, error) {
	loggers := []*options.LoggerConfig{}
	for _, logger := range opts.Loggers {
		exportedLogger, err := logger.Export()
		if err != nil {
			return options.Output{}, fmt.Errorf("problem exporting logger config: %w", err)
		}
		loggers = append(loggers, exportedLogger)
	}
	return options.Output{
		SuppressOutput:    opts.SuppressOutput,
		SuppressError:     opts.SuppressError,
		SendOutputToError: opts.RedirectOutputToError,
		SendErrorToOutput: opts.RedirectErrorToOutput,
		Loggers:           loggers,
	}, nil
}

// ConvertOutputOptions takes a Jasper OutputOptions struct and returns an
// equivalent protobuf RPC OutputOptions struct. ConvertOutputOptions is the
// inverse of (OutputOptions) Export().
func ConvertOutputOptions(opts options.Output) (OutputOptions, error) {
	loggers := []*LoggerConfig{}
	for _, logger := range opts.Loggers {
		convertedLoggerConfig, err := ConvertLoggerConfig(logger)
		if err != nil {
			return OutputOptions{}, fmt.Errorf("problem converting logger config: %w", err)
		}
		loggers = append(loggers, convertedLoggerConfig)
	}
	return OutputOptions{
		SuppressOutput:        opts.SuppressOutput,
		SuppressError:         opts.SuppressError,
		RedirectOutputToError: opts.SendOutputToError,
		RedirectErrorToOutput: opts.SendErrorToOutput,
		Loggers:               loggers,
	}, nil
}

// Export takes a protobuf RPC Logger struct and returns the analogous
// Jasper Logger struct.
func (logger LoggerConfig) Export() (*options.LoggerConfig, error) {
	var producer options.LoggerProducer
	switch {
	case logger.GetDefault() != nil:
		producer = logger.GetDefault().Export()
	case logger.GetFile() != nil:
		producer = logger.GetFile().Export()
	case logger.GetInherited() != nil:
		producer = logger.GetInherited().Export()
	case logger.GetInMemory() != nil:
		producer = logger.GetInMemory().Export()
	case logger.GetSplunk() != nil:
		producer = logger.GetSplunk().Export()
	case logger.GetRaw() != nil:
		return logger.GetRaw().Export()
	}
	if producer == nil {
		return nil, errors.New("logger config options invalid")
	}

	config := &options.LoggerConfig{}
	return config, config.Set(producer)
}

// ConvertLoggerConfig takes a Jasper options.LoggerConfig struct and returns
// an equivalent protobuf RPC LoggerConfig struct. ConvertLoggerConfig is the
// inverse of (LoggerConfig) Export().
func ConvertLoggerConfig(config *options.LoggerConfig) (*LoggerConfig, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("problem marshalling logger config: %w", err)
	}

	return &LoggerConfig{
		Producer: &LoggerConfig_Raw{
			Raw: &RawLoggerConfig{
				Format:     ConvertRawLoggerConfigFormat(options.RawLoggerConfigFormatJSON),
				ConfigData: data,
			},
		},
	}, nil
}

// Export takes a protobuf RPC LogLevel struct and returns the analogous send
// LevelInfo struct.
func (l *LogLevel) Export() level.Priority {
	return level.Priority(l.Threshold)
}

// ConvertLogLevel takes a send LevelInfo struct and returns an equivalent
// protobuf RPC LogLevel struct. ConvertLogLevel is the inverse of
// (*LogLevel) Export().
func ConvertLogLevel(l level.Priority) *LogLevel {
	return &LogLevel{Threshold: int32(l)}

}

// Export takes a protobuf RPC BufferOptions struct and returns the analogous
// Jasper BufferOptions struct.
func (opts *BufferOptions) Export() options.BufferOptions {
	return options.BufferOptions{
		Buffered: opts.Buffered,
		Duration: time.Duration(opts.Duration),
		MaxSize:  int(opts.MaxSize),
	}
}

// ConvertBufferOptions takes a Jasper BufferOptions struct and returns an
// equivalent protobuf RPC BufferOptions struct. ConvertBufferOptions is the
// inverse of (*BufferOptions) Export().
func ConvertBufferOptions(opts options.BufferOptions) *BufferOptions {
	return &BufferOptions{
		Buffered: opts.Buffered,
		Duration: int64(opts.Duration),
		MaxSize:  int64(opts.MaxSize),
	}
}

// Export takes a protobuf RPC LogFormat struct and returns the analogous
// Jasper LogFormat struct.
func (f LogFormat) Export() options.LogFormat {
	switch f {
	case LogFormat_LOGFORMATDEFAULT:
		return options.LogFormatDefault
	case LogFormat_LOGFORMATJSON:
		return options.LogFormatJSON
	case LogFormat_LOGFORMATPLAIN:
		return options.LogFormatPlain
	default:
		return options.LogFormatInvalid
	}
}

// ConvertLogFormat takes a Jasper LogFormat struct and returns an
// equivalent protobuf RPC LogFormat struct. ConvertLogFormat is the
// inverse of (LogFormat) Export().
func ConvertLogFormat(f options.LogFormat) LogFormat {
	switch f {
	case options.LogFormatDefault:
		return LogFormat_LOGFORMATDEFAULT
	case options.LogFormatJSON:
		return LogFormat_LOGFORMATJSON
	case options.LogFormatPlain:
		return LogFormat_LOGFORMATPLAIN
	default:
		return LogFormat_LOGFORMATUNKNOWN
	}
}

// Export takes a protobuf RPC BaseOptions struct and returns the analogous
// Jasper BaseOptions struct.
func (opts BaseOptions) Export() options.BaseOptions {
	return options.BaseOptions{
		Level:  opts.Level.Export(),
		Buffer: opts.Buffer.Export(),
		Format: opts.Format.Export(),
	}
}

// Export takes a protobuf RPC DefaultLoggerOptions struct and returns the
// analogous Jasper options.LoggerProducer.
func (opts DefaultLoggerOptions) Export() options.LoggerProducer {
	return &options.DefaultLoggerOptions{
		Prefix: opts.Prefix,
		Base:   opts.Base.Export(),
	}
}

// Export takes a protobuf RPC FileLoggerOptions struct and returns the
// analogous Jasper options.LoggerProducer.
func (opts FileLoggerOptions) Export() options.LoggerProducer {
	return &options.FileLoggerOptions{
		Filename: opts.Filename,
		Base:     opts.Base.Export(),
	}
}

// Export takes a protobuf RPC InheritedLoggerOptions struct and returns the
// analogous Jasper options.LoggerProducer.
func (opts InheritedLoggerOptions) Export() options.LoggerProducer {
	return &options.InheritedLoggerOptions{
		Base: opts.Base.Export(),
	}
}

// Export takes a protobuf RPC InMemoryLoggerOptions struct and returns the
// analogous Jasper options.LoggerProducer.
func (opts InMemoryLoggerOptions) Export() options.LoggerProducer {
	return &options.InMemoryLoggerOptions{
		InMemoryCap: int(opts.InMemoryCap),
		Base:        opts.Base.Export(),
	}
}

// Export takes a protobuf RPC SplunkInfo struct and returns the analogous
// grip send.SplunkConnectionInfo struct.
func (opts SplunkInfo) Export() splunk.ConnectionInfo {
	return splunk.ConnectionInfo{
		ServerURL: opts.Url,
		Token:     opts.Token,
		Channel:   opts.Channel,
	}
}

// ConvertSplunkInfo takes a grip send.SplunkConnectionInfo and returns the
// analogous protobuf RPC SplunkInfo struct. ConvertSplunkInfo is the inverse
// of (SplunkInfo) Export().
func ConvertSplunkInfo(opts splunk.ConnectionInfo) *SplunkInfo {
	return &SplunkInfo{
		Url:     opts.ServerURL,
		Token:   opts.Token,
		Channel: opts.Channel,
	}
}

// Export takes a protobuf RPC SplunkLoggerOptions struct and returns the
// analogous Jasper options.LoggerProducer.
func (opts SplunkLoggerOptions) Export() options.LoggerProducer {
	return &jsplunk.LoggerOptions{
		Splunk: opts.Splunk.Export(),
		Base:   opts.Base.Export(),
	}
}

// Export takes a protobuf RPC RawLoggerConfigFormat enum and returns the
// analogous Jasper options.RawLoggerConfigFormat type.
func (f RawLoggerConfigFormat) Export() options.RawLoggerConfigFormat {
	switch f {
	case RawLoggerConfigFormat_RAWLOGGERCONFIGFORMATJSON:
		return options.RawLoggerConfigFormatJSON
	default:
		return options.RawLoggerConfigFormatInvalid
	}
}

// ConvertRawLoggerConfigFormat takes a Jasper RawLoggerConfigFormat type and
// returns an equivalent protobuf RPC RawLoggerConfigFormat enum.
// ConvertLogFormat is the inverse of (RawLoggerConfigFormat) Export().
func ConvertRawLoggerConfigFormat(f options.RawLoggerConfigFormat) RawLoggerConfigFormat {
	switch f {
	case options.RawLoggerConfigFormatJSON:
		return RawLoggerConfigFormat_RAWLOGGERCONFIGFORMATJSON
	default:
		return RawLoggerConfigFormat_RAWLOGGERCONFIGFORMATUNKNOWN
	}
}

// Export takes a protobuf RPC RawLoggerConfig struct and returns the
// analogous Jasper options.LoggerConfig
func (logger RawLoggerConfig) Export() (*options.LoggerConfig, error) {
	config := &options.LoggerConfig{}
	if err := logger.Format.Export().Unmarshal(logger.ConfigData, config); err != nil {
		return nil, fmt.Errorf("problem unmarshalling raw config: %w", err)
	}
	return config, nil
}

// Export takes a protobuf RPC DownloadInfo struct and returns the analogous
// options.Download struct.
func (opts *DownloadInfo) Export() roptions.Download {
	return roptions.Download{
		Path:        opts.Path,
		URL:         opts.Url,
		ArchiveOpts: opts.ArchiveOpts.Export(),
	}
}

// ConvertDownloadOptions takes an remote.Download struct and returns an
// equivalent protobuf RPC DownloadInfo struct. ConvertDownloadOptions is the
// inverse of (*DownloadInfo) Export().
func ConvertDownloadOptions(opts roptions.Download) *DownloadInfo {
	return &DownloadInfo{
		Path:        opts.Path,
		Url:         opts.URL,
		ArchiveOpts: ConvertArchiveOptions(opts.ArchiveOpts),
	}
}

// Export takes a protobuf RPC WriteFileInfo struct and returns the analogous
// options.WriteFile struct.
func (opts *WriteFileInfo) Export() options.WriteFile {
	return options.WriteFile{
		Path:    opts.Path,
		Content: opts.Content,
		Append:  opts.Append,
		Perm:    os.FileMode(opts.Perm),
	}
}

// ConvertWriteFileOptions takes an options.WriteFile struct and returns an
// equivalent protobuf RPC WriteFileInfo struct. ConvertWriteFileOptions is the
// inverse of (*WriteFileInfo) Export().
func ConvertWriteFileOptions(opts options.WriteFile) *WriteFileInfo {
	return &WriteFileInfo{
		Path:    opts.Path,
		Content: opts.Content,
		Append:  opts.Append,
		Perm:    uint32(opts.Perm),
	}
}

// Export takes a protobuf RPC ArchiveFormat struct and returns the analogous
// Jasper ArchiveFormat struct.
func (format ArchiveFormat) Export() roptions.ArchiveFormat {
	switch format {
	case ArchiveFormat_ARCHIVEAUTO:
		return roptions.ArchiveAuto
	case ArchiveFormat_ARCHIVETARGZ:
		return roptions.ArchiveTarGz
	case ArchiveFormat_ARCHIVEZIP:
		return roptions.ArchiveZip
	default:
		return roptions.ArchiveFormat("")
	}
}

// ConvertArchiveFormat takes a Jasper ArchiveFormat struct and returns an
// equivalent protobuf RPC ArchiveFormat struct. ConvertArchiveFormat is the
// inverse of (ArchiveFormat) Export().
func ConvertArchiveFormat(format roptions.ArchiveFormat) ArchiveFormat {
	switch format {
	case roptions.ArchiveAuto:
		return ArchiveFormat_ARCHIVEAUTO
	case roptions.ArchiveTarGz:
		return ArchiveFormat_ARCHIVETARGZ
	case roptions.ArchiveZip:
		return ArchiveFormat_ARCHIVEZIP
	default:
		return ArchiveFormat_ARCHIVEUNKNOWN
	}
}

// Export takes a protobuf RPC ArchiveOptions struct and returns the analogous
// Jasper ArchiveOptions struct.
func (opts ArchiveOptions) Export() roptions.Archive {
	return roptions.Archive{
		ShouldExtract: opts.ShouldExtract,
		Format:        opts.Format.Export(),
		TargetPath:    opts.TargetPath,
	}
}

// ConvertArchiveOptions takes a Jasper ArchiveOptions struct and returns an
// equivalent protobuf RPC ArchiveOptions struct. ConvertArchiveOptions is the
// inverse of (ArchiveOptions) Export().
func ConvertArchiveOptions(opts roptions.Archive) *ArchiveOptions {
	return &ArchiveOptions{
		ShouldExtract: opts.ShouldExtract,
		Format:        ConvertArchiveFormat(opts.Format),
		TargetPath:    opts.TargetPath,
	}
}

// Export takes a protobuf RPC SignalTriggerParams struct and returns the analogous
// Jasper process ID and SignalTriggerID.
func (t SignalTriggerParams) Export() (string, jasper.SignalTriggerID) {
	return t.ProcessID.Value, t.SignalTriggerID.Export()
}

// ConvertSignalTriggerParams takes a Jasper process ID and a SignalTriggerID
// and returns an equivalent protobuf RPC SignalTriggerParams struct.
// ConvertSignalTriggerParams is the inverse of (SignalTriggerParams) Export().
func ConvertSignalTriggerParams(jasperProcessID string, signalTriggerID jasper.SignalTriggerID) *SignalTriggerParams {
	return &SignalTriggerParams{
		ProcessID:       &JasperProcessID{Value: jasperProcessID},
		SignalTriggerID: ConvertSignalTriggerID(signalTriggerID),
	}
}

// Export takes a protobuf RPC SignalTriggerID and returns the analogous
// Jasper SignalTriggerID.
func (t SignalTriggerID) Export() jasper.SignalTriggerID {
	switch t {
	case SignalTriggerID_CLEANTERMINATION:
		return jasper.CleanTerminationSignalTrigger
	default:
		return jasper.SignalTriggerID("")
	}
}

// ConvertSignalTriggerID takes a Jasper SignalTriggerID and returns an
// equivalent protobuf RPC SignalTriggerID. ConvertSignalTrigger is the
// inverse of (SignalTriggerID) Export().
func ConvertSignalTriggerID(id jasper.SignalTriggerID) SignalTriggerID {
	switch id {
	case jasper.CleanTerminationSignalTrigger:
		return SignalTriggerID_CLEANTERMINATION
	default:
		return SignalTriggerID_NONE
	}
}

// Export takes a protobuf RPC LogStream and returns the analogous
// Jasper LogStream.
func (l *LogStream) Export() jasper.LogStream {
	return jasper.LogStream{
		Logs: l.Logs,
		Done: l.Done,
	}
}

// ConvertLogStream takes a Jasper LogStream and returns an
// equivalent protobuf RPC LogStream. ConvertLogStream is the
// inverse of (*LogStream) Export().
func ConvertLogStream(l jasper.LogStream) *LogStream {
	return &LogStream{
		Logs: l.Logs,
		Done: l.Done,
	}
}

// Export takes a protobuf RPC ScriptingOptions and returns the analogous
// ScriptingHarness options.
func (o *ScriptingOptions) Export() (options.ScriptingHarness, error) {
	switch val := o.Value.(type) {
	case *ScriptingOptions_Golang:
		output, err := o.Output.Export()
		if err != nil {
			return nil, fmt.Errorf("problem exporting output options: %w", err)
		}
		return &options.ScriptingGolang{
			Gopath:         val.Golang.Gopath,
			Goroot:         val.Golang.Goroot,
			Packages:       val.Golang.Packages,
			Directory:      val.Golang.Directory,
			UpdatePackages: val.Golang.UpdatePackages,
			CachedDuration: time.Duration(o.Duration),
			Environment:    o.Environment,
			Output:         output,
		}, nil
	case *ScriptingOptions_Python:
		output, err := o.Output.Export()
		if err != nil {
			return nil, fmt.Errorf("problem exporting output options: %w", err)
		}
		return &options.ScriptingPython{
			VirtualEnvPath:      val.Python.VirtualEnvPath,
			RequirementsPath:    val.Python.RequirementsPath,
			InterpreterBinary:   val.Python.InterpreterBinary,
			Packages:            val.Python.Packages,
			LegacyPython:        val.Python.LegacyPython,
			AddTestRequirements: val.Python.AddTestReqs,
			CachedDuration:      time.Duration(o.Duration),
			Environment:         o.Environment,
			Output:              output,
		}, nil
	case *ScriptingOptions_Roswell:
		output, err := o.Output.Export()
		if err != nil {
			return nil, fmt.Errorf("problem exporting output options: %w", err)
		}
		return &options.ScriptingRoswell{
			Path:           val.Roswell.Path,
			Systems:        val.Roswell.Systems,
			Lisp:           val.Roswell.Lisp,
			CachedDuration: time.Duration(o.Duration),
			Environment:    o.Environment,
			Output:         output,
		}, nil
	default:
		return nil, fmt.Errorf("invalid scripting options type %T", val)
	}
}

// ConvertScriptingOptions takes ScriptingHarness options and returns an
// equivalent protobuf RPC ScriptingOptions. ConvertScriptingOptions is the
// inverse of (*ScriptingOptions) Export().
func ConvertScriptingOptions(opts options.ScriptingHarness) (*ScriptingOptions, error) {
	switch val := opts.(type) {
	case *options.ScriptingGolang:
		out, err := ConvertOutputOptions(val.Output)
		if err != nil {
			return nil, fmt.Errorf("problem converting output options: %w", err)
		}
		return &ScriptingOptions{
			Duration:    int64(val.CachedDuration),
			Environment: val.Environment,
			Output:      &out,
			Value: &ScriptingOptions_Golang{
				Golang: &ScriptingOptionsGolang{
					Gopath:         val.Gopath,
					Goroot:         val.Goroot,
					Packages:       val.Packages,
					Directory:      val.Directory,
					UpdatePackages: val.UpdatePackages,
				},
			},
		}, nil
	case *options.ScriptingPython:
		out, err := ConvertOutputOptions(val.Output)
		if err != nil {
			return nil, fmt.Errorf("problem converting output options: %w", err)
		}
		return &ScriptingOptions{
			Duration:    int64(val.CachedDuration),
			Environment: val.Environment,
			Output:      &out,
			Value: &ScriptingOptions_Python{
				Python: &ScriptingOptionsPython{
					VirtualEnvPath:    val.VirtualEnvPath,
					RequirementsPath:  val.RequirementsPath,
					InterpreterBinary: val.InterpreterBinary,
					Packages:          val.Packages,
					LegacyPython:      val.LegacyPython,
					AddTestReqs:       val.AddTestRequirements,
				},
			},
		}, nil
	case *options.ScriptingRoswell:
		out, err := ConvertOutputOptions(val.Output)
		if err != nil {
			return nil, fmt.Errorf("problem converting output options: %w", err)
		}
		return &ScriptingOptions{
			Duration:    int64(val.CachedDuration),
			Environment: val.Environment,
			Output:      &out,
			Value: &ScriptingOptions_Roswell{
				Roswell: &ScriptingOptionsRoswell{
					Path:    val.Path,
					Systems: val.Systems,
					Lisp:    val.Lisp,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("scripting options for '%T' is not supported", opts)
	}
}

// ConvertScriptingTestResults takes scripting TestResults and returns an
// equivalent protobuf RPC ScriptingHarnessTestResult.
func ConvertScriptingTestResults(res []scripting.TestResult) []*ScriptingHarnessTestResult {
	out := make([]*ScriptingHarnessTestResult, len(res))
	for idx, r := range res {
		out[idx] = &ScriptingHarnessTestResult{
			Name:     r.Name,
			StartAt:  timestamppb.New(r.StartAt),
			Duration: durationpb.New(r.Duration),
			Outcome:  string(r.Outcome),
		}
	}
	return out
}

// Export takes a protobuf RPC ScriptingHarnessTestResponse and returns the
// analogous scripting TestResult.
func (r *ScriptingHarnessTestResponse) Export() ([]scripting.TestResult, error) {
	out := make([]scripting.TestResult, len(r.Results))
	for idx, res := range r.Results {
		var startAt time.Time
		if res.StartAt != nil {
			startAt = res.StartAt.AsTime()
		}
		var duration time.Duration
		if res.Duration != nil {
			duration = res.Duration.AsDuration()
		}

		out[idx] = scripting.TestResult{
			Name:     res.Name,
			StartAt:  startAt,
			Duration: duration,
			Outcome:  scripting.TestOutcome(res.Outcome),
		}
	}
	return out, nil
}

// Export takes a protobuf RPC ScriptingHarnessTestArgs and returns the
// analogous scripting TestOptions.
func (a *ScriptingHarnessTestArgs) Export() ([]scripting.TestOptions, error) {
	out := make([]scripting.TestOptions, len(a.Options))
	for idx, opts := range a.Options {
		out[idx] = scripting.TestOptions{
			Name:    opts.Name,
			Args:    opts.Args,
			Pattern: opts.Pattern,
			Timeout: opts.Timeout.AsDuration(),
			Count:   int(opts.Count),
		}
	}
	return out, nil
}

// ConvertScriptingTestResults takes scripting TestOptions and returns an
// equivalent protobuf RPC ScriptingHarnessTestOptions.
func ConvertScriptingTestOptions(args []scripting.TestOptions) []*ScriptingHarnessTestOptions {
	out := make([]*ScriptingHarnessTestOptions, len(args))
	for idx, opt := range args {
		out[idx] = &ScriptingHarnessTestOptions{
			Name:    opt.Name,
			Args:    opt.Args,
			Pattern: opt.Pattern,
			Timeout: durationpb.New(opt.Timeout),
			Count:   int32(opt.Count),
		}
	}
	return out
}

// Export takes a protobuf RPC LoggingPayloadFormat and returns the
// analogous LoggingPayloadFormat.
func (lf LoggingPayloadFormat) Export() options.LoggingPayloadFormat {
	switch lf {
	case LoggingPayloadFormat_FORMATJSON:
		return options.LoggingPayloadFormatJSON
	case LoggingPayloadFormat_FORMATSTRING:
		return options.LoggingPayloadFormatSTRING
	default:
		return ""
	}
}

// ConvertLoggingPayloadFormat takes LoggingPayloadFormat options and returns an
// equivalent protobuf RPC LoggingPayloadFormat. ConvertLoggingPayloadFormat is
// the inverse of (LoggingPayloadFormat) Export().
func ConvertLoggingPayloadFormat(in options.LoggingPayloadFormat) LoggingPayloadFormat {
	switch in {
	case options.LoggingPayloadFormatJSON:
		return LoggingPayloadFormat_FORMATJSON
	case options.LoggingPayloadFormatSTRING:
		return LoggingPayloadFormat_FORMATSTRING
	default:
		return 0
	}
}

// Export takes a protobuf RPC LoggingPayload and returns the
// analogous LoggingPayload options.
func (lp *LoggingPayload) Export() *options.LoggingPayload {
	data := make([]interface{}, len(lp.Data))
	for idx := range lp.Data {
		switch val := lp.Data[idx].Data.(type) {
		case *LoggingPayloadData_Msg:
			data[idx] = val.Msg
		case *LoggingPayloadData_Raw:
			data[idx] = val.Raw
		}
	}

	return &options.LoggingPayload{
		Data:              data,
		LoggerID:          lp.LoggerID,
		IsMulti:           lp.IsMulti,
		PreferSendToError: lp.PreferSendToError,
		AddMetadata:       lp.AddMetadata,
		Priority:          level.Priority(lp.Priority),
		Format:            lp.Format.Export(),
	}
}

func convertMessage(format options.LoggingPayloadFormat, m interface{}) *LoggingPayloadData {
	out := &LoggingPayloadData{}

	switch m := m.(type) {
	case message.Composer:
		switch format {
		case options.LoggingPayloadFormatSTRING:
			out.Data = &LoggingPayloadData_Msg{Msg: m.String()}
		case options.LoggingPayloadFormatJSON:
			payload, _ := json.Marshal(m.Raw())
			out.Data = &LoggingPayloadData_Raw{Raw: payload}
		default:
			out.Data = &LoggingPayloadData_Raw{}
		}
	case string:
		switch format {
		case options.LoggingPayloadFormatJSON:
			out.Data = &LoggingPayloadData_Raw{Raw: []byte(m)}
		default:
			out.Data = &LoggingPayloadData_Msg{Msg: m}
		}
	case []byte:
		switch format {
		case options.LoggingPayloadFormatSTRING:
			out.Data = &LoggingPayloadData_Msg{Msg: string(m)}
		default:
			out.Data = &LoggingPayloadData_Raw{Raw: m}
		}
	default:
		out.Data = &LoggingPayloadData_Raw{}
	}
	return out
}

// ConvertLoggingPayload takes LoggingPayload options and returns an
// equivalent protobuf RPC LoggingPayload. ConvertLoggingPayload is
// the inverse of (*LoggingPayload) Export().
func ConvertLoggingPayload(in options.LoggingPayload) *LoggingPayload {
	data := []*LoggingPayloadData{}
	switch val := in.Data.(type) {
	case []interface{}:
		for idx := range val {
			data = append(data, convertMessage(in.Format, val[idx]))
		}
	case []string:
		for idx := range val {
			data = append(data, convertMessage(in.Format, val[idx]))
		}
	case [][]byte:
		for idx := range val {
			data = append(data, convertMessage(in.Format, val[idx]))
		}
	case []message.Composer:
		for idx := range val {
			data = append(data, convertMessage(in.Format, val[idx]))
		}
	case *message.GroupComposer:
		msgs := val.Messages()
		for idx := range msgs {
			data = append(data, convertMessage(in.Format, msgs[idx]))
		}
	case string:
		data = append(data, convertMessage(in.Format, val))
	case []byte:
		data = append(data, convertMessage(in.Format, val))
	}

	return &LoggingPayload{
		LoggerID:          in.LoggerID,
		Priority:          int32(in.Priority),
		IsMulti:           in.IsMulti,
		PreferSendToError: in.PreferSendToError,
		AddMetadata:       in.AddMetadata,
		Format:            ConvertLoggingPayloadFormat(in.Format),
		Data:              data,
	}
}

// Export takes a protobuf RPC LoggingCacheInstance and returns the
// analogous CacheLogger options.
func (l *LoggingCacheInstance) Export() (*options.CachedLogger, error) {
	if !l.Outcome.Success {
		return nil, errors.New(l.Outcome.Text)
	}

	return &options.CachedLogger{
		Accessed: l.Accessed.AsTime(),
		ID:       l.Id,
		Manager:  l.Manager,
	}, nil
}

// ConvertCachedLogger takes CachedLogger options and returns an
// equivalent protobuf RPC LoggingCacheInstance. ConvertLoggingPayload is
// the inverse of (*LoggingCacheInstance) Export().
func ConvertCachedLogger(opts *options.CachedLogger) *LoggingCacheInstance {
	return &LoggingCacheInstance{
		Outcome: &OperationOutcome{
			Success: true,
		},
		Id:       opts.ID,
		Manager:  opts.Manager,
		Accessed: timestamppb.New(opts.Accessed),
	}
}

// ConvertLoggingCreateArgs takes the given ID and returns an equivalent
// protobuf RPC LoggingCacheCreateArgs.
func ConvertLoggingCreateArgs(id string, opts *options.Output) (*LoggingCacheCreateArgs, error) {
	o, err := ConvertOutputOptions(*opts)
	if err != nil {
		return nil, fmt.Errorf("problem converting output options: %w", err)
	}
	return &LoggingCacheCreateArgs{
		Name:    id,
		Options: &o,
	}, nil
}
