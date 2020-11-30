package remote

import (
	"context"
	"io"
	"net"
	"strconv"

	"github.com/deciduosity/birch"
	"github.com/cdr/grip"
	"github.com/cdr/grip/recovery"
	"github.com/deciduosity/jasper"
	"github.com/deciduosity/jasper/options"
	"github.com/deciduosity/jasper/scripting"
	"github.com/deciduosity/jasper/util"
	"github.com/deciduosity/mrpc"
	"github.com/deciduosity/mrpc/mongowire"
	"github.com/deciduosity/mrpc/shell"
	"github.com/pkg/errors"
)

type mdbService struct {
	mrpc.Service
	manager      jasper.Manager
	harnessCache scripting.HarnessCache
	marshaler    options.Marshaler
	unmarshaler  options.Unmarshaler
}

// StartMDBService wraps an existing Jasper manager in a MongoDB wire protocol
// service and starts it. The caller is responsible for closing the connection
// using the returned jasper.CloseFunc.
func StartMDBService(ctx context.Context, m jasper.Manager, addr net.Addr) (util.CloseFunc, error) { //nolint: interfacer
	host, p, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, errors.Wrap(err, "port is not a number")
	}

	baseSvc, err := shell.NewShellService(host, port)
	if err != nil {
		return nil, errors.Wrap(err, "could not create base service")
	}
	svc := &mdbService{
		Service:      baseSvc,
		manager:      m,
		harnessCache: scripting.NewCache(),
		unmarshaler:  options.GetGlobalLoggerRegistry().Unmarshaler(options.RawLoggerConfigFormatBSON),
		marshaler:    options.GetGlobalLoggerRegistry().Marshaler(options.RawLoggerConfigFormatBSON),
	}
	if err := svc.registerHandlers(); err != nil {
		return nil, errors.Wrap(err, "error registering handlers")
	}

	cctx, ccancel := context.WithCancel(context.Background())
	go func() {
		defer func() {
			grip.Error(recovery.HandlePanicWithError(recover(), nil, "running wire service"))
		}()
		grip.Notice(svc.Run(cctx))
	}()

	return func() error { ccancel(); return nil }, nil
}

func (s *mdbService) registerHandlers() error {
	for name, handler := range map[string]mrpc.HandlerFunc{
		// Manager commands
		ManagerIDCommand:     s.managerID,
		CreateProcessCommand: s.managerCreateProcess,
		ListCommand:          s.managerList,
		GroupCommand:         s.managerGroup,
		GetProcessCommand:    s.managerGetProcess,
		ClearCommand:         s.managerClear,
		CloseCommand:         s.managerClose,
		WriteFileCommand:     s.managerWriteFile,

		// Process commands
		InfoCommand:                    s.processInfo,
		RunningCommand:                 s.processRunning,
		CompleteCommand:                s.processComplete,
		WaitCommand:                    s.processWait,
		SignalCommand:                  s.processSignal,
		RegisterSignalTriggerIDCommand: s.processRegisterSignalTriggerID,
		RespawnCommand:                 s.processRespawn,
		TagCommand:                     s.processTag,
		GetTagsCommand:                 s.processGetTags,
		ResetTagsCommand:               s.processResetTags,

		// Scripting commands
		ScriptingGetCommand:       s.scriptingGet,
		ScriptingCreateCommand:    s.scriptingCreate,
		ScriptingSetupCommand:     s.scriptingSetup,
		ScriptingCleanupCommand:   s.scriptingCleanup,
		ScriptingRunCommand:       s.scriptingRun,
		ScriptingRunScriptCommand: s.scriptingRunScript,
		ScriptingBuildCommand:     s.scriptingBuild,
		ScriptingTestCommand:      s.scriptingTest,

		// Logging Commands
		LoggingCacheSizeCommand:   s.loggingSize,
		LoggingCacheCreateCommand: s.loggingCreate,
		LoggingCacheDeleteCommand: s.loggingDelete,
		LoggingCacheGetCommand:    s.loggingGet,
		LoggingCachePruneCommand:  s.loggingPrune,
		LoggingSendMessageCommand: s.loggingSendMessage,

		// Remote client commands
		DownloadFileCommand: s.downloadFile,
		GetLogStreamCommand: s.getLogStream,
		SignalEventCommand:  s.signalEvent,
	} {
		if err := s.RegisterOperation(&mongowire.OpScope{
			Type:    mongowire.OP_COMMAND,
			Command: name,
		}, handler); err != nil {
			return errors.Wrapf(err, "could not register handler for %s", name)
		}
	}

	return nil
}

// Constants representing remote client commands.
const (
	DownloadFileCommand = "download_file"
	GetLogStreamCommand = "get_log_stream"
	SignalEventCommand  = "signal_event"
)

func (s *mdbService) readRequest(msg mongowire.Message, in interface{}) error {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		return errors.Wrap(err, "could not read response")
	}

	data, err := doc.MarshalBSON()
	if err != nil {
		return errors.Wrap(err, "could not read response data")
	}

	if err := s.unmarshaler(data, in); err != nil {
		return errors.Wrap(err, "problem parsing response body")

	}

	return nil
}

func (s *mdbService) downloadFile(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := downloadFileRequest{}

	if err := s.readRequest(msg, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), DownloadFileCommand)
		return
	}

	opts := req.Options

	if err := opts.Validate(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "invalid download options"), DownloadFileCommand)
		return
	}

	if err := opts.Download(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not download file"), DownloadFileCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, DownloadFileCommand)
}

func (s *mdbService) getLogStream(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := getLogStreamRequest{}
	if err := s.readRequest(msg, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), GetLogStreamCommand)
		return
	}

	id := req.Params.ID
	count := req.Params.Count

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), GetLogStreamCommand)
		return
	}

	var done bool
	logs, err := jasper.GetInMemoryLogStream(ctx, proc, count)
	if err == io.EOF {
		done = true
	} else if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get logs"), GetLogStreamCommand)
		return
	}

	payload, err := s.makePayload(makeGetLogStreamResponse(logs, done))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), GetLogStreamCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), GetLogStreamCommand)
		return
	}

	shell.WriteResponse(ctx, w, resp, GetLogStreamCommand)
}

func (s *mdbService) makePayload(in interface{}) (*birch.Document, error) {
	data, err := s.marshaler(in)
	if err != nil {
		return nil, errors.Wrap(err, "problem producing data")
	}
	return birch.ReadDocument(data)
}

func (s *mdbService) signalEvent(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := signalEventRequest{}
	if err := s.readRequest(msg, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), SignalEventCommand)
		return
	}

	name := req.Name

	if err := jasper.SignalEvent(ctx, name); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrapf(err, "could not signal event '%s'", name), SignalEventCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, SignalEventCommand)
}
