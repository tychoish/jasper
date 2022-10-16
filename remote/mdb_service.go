package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/tychoish/birch"
	"github.com/tychoish/birch/mrpc"
	"github.com/tychoish/birch/mrpc/mongowire"
	"github.com/tychoish/birch/mrpc/shell"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/recovery"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
	"github.com/tychoish/jasper/util"
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
func StartMDBService(ctx context.Context, m jasper.Manager, addr net.Addr) (util.CloseFunc, error) {
	host, p, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, fmt.Errorf("port is not a number: %w", err)
	}

	baseSvc, err := shell.NewShellService(host, port)
	if err != nil {
		return nil, fmt.Errorf("could not create base service: %w", err)
	}
	svc := &mdbService{
		Service:      baseSvc,
		manager:      m,
		harnessCache: scripting.NewCache(),
		unmarshaler:  options.GetGlobalLoggerRegistry().Unmarshaler(options.RawLoggerConfigFormatBSON),
		marshaler:    options.GetGlobalLoggerRegistry().Marshaler(options.RawLoggerConfigFormatBSON),
	}
	if err := svc.registerHandlers(); err != nil {
		return nil, fmt.Errorf("error registering handlers: %w", err)
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
		LoggingCacheSizeCommand:    s.loggingSize,
		LoggingCacheCreateCommand:  s.loggingCreate,
		LoggingCacheDeleteCommand:  s.loggingDelete,
		LoggingCacheGetCommand:     s.loggingGet,
		LoggingCachePruneCommand:   s.loggingPrune,
		LoggingSendMessagesCommand: s.loggingSendMessages,

		// Remote client commands
		DownloadFileCommand: s.downloadFile,
		GetLogStreamCommand: s.getLogStream,
		SignalEventCommand:  s.signalEvent,
	} {
		if err := s.RegisterOperation(&mongowire.OpScope{
			Type:    mongowire.OP_COMMAND,
			Command: name,
		}, handler); err != nil {
			return fmt.Errorf("could not register handler for %q: %w", name, err)
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
		return fmt.Errorf("could not read response: %w", err)
	}

	data, err := doc.MarshalBSON()
	if err != nil {
		return fmt.Errorf("could not read response data: %w", err)
	}

	if err := s.unmarshaler(data, in); err != nil {
		return fmt.Errorf("problem parsing response body: %w", err)

	}

	return nil
}

func (s *mdbService) downloadFile(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := downloadFileRequest{}

	if err := s.readRequest(msg, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), DownloadFileCommand)
		return
	}

	opts := req.Options

	if err := opts.Validate(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("invalid download options: %w", err), DownloadFileCommand)
		return
	}

	if err := opts.Download(ctx); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not download file: %w", err), DownloadFileCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, DownloadFileCommand)
}

func (s *mdbService) getLogStream(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := getLogStreamRequest{}
	if err := s.readRequest(msg, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), GetLogStreamCommand)
		return
	}

	id := req.Params.ID
	count := req.Params.Count

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), GetLogStreamCommand)
		return
	}

	var done bool
	logs, err := jasper.GetInMemoryLogStream(ctx, proc, count)
	if err == io.EOF {
		done = true
	} else if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get logs: %w", err), GetLogStreamCommand)
		return
	}

	payload, err := s.makePayload(makeGetLogStreamResponse(logs, done))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), GetLogStreamCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), GetLogStreamCommand)
		return
	}

	shell.WriteResponse(ctx, w, resp, GetLogStreamCommand)
}

func (s *mdbService) makePayload(in interface{}) (*birch.Document, error) {
	data, err := s.marshaler(in)
	if err != nil {
		return nil, fmt.Errorf("problem producing data: %w", err)
	}
	return birch.ReadDocument(data)
}

func (s *mdbService) signalEvent(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := signalEventRequest{}
	if err := s.readRequest(msg, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), SignalEventCommand)
		return
	}

	name := req.Name

	if err := jasper.SignalEvent(ctx, name); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not signal event %q: %w", name, err), SignalEventCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, SignalEventCommand)
}
