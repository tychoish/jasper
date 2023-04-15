package remote

import (
	"context"
	"fmt"
	"io"
	"syscall"

	"github.com/tychoish/birch/x/mrpc/mongowire"
	"github.com/tychoish/birch/x/mrpc/shell"
	"github.com/tychoish/jasper"
)

// Constants representing process commands.
const (
	ProcessIDCommand               = "process_id"
	InfoCommand                    = "info"
	RunningCommand                 = "running"
	CompleteCommand                = "complete"
	WaitCommand                    = "wait"
	RespawnCommand                 = "respawn"
	SignalCommand                  = "signal"
	RegisterSignalTriggerIDCommand = "register_signal_trigger_id"
	GetTagsCommand                 = "get_tags"
	TagCommand                     = "add_tag"
	ResetTagsCommand               = "reset_tags"
)

func (s *mdbService) processInfo(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), InfoCommand)
		return
	}
	req := infoRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), InfoCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), InfoCommand)
		return
	}

	payload, err := s.makePayload(makeInfoResponse(proc.Info(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), InfoCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), InfoCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, InfoCommand)
}

func (s *mdbService) processRunning(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), RunningCommand)
		return
	}
	req := runningRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), RunningCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), RunningCommand)
		return
	}

	payload, err := s.makePayload(makeRunningResponse(proc.Running(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), RunningCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), RunningCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, RunningCommand)
}

func (s *mdbService) processComplete(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), CompleteCommand)
		return
	}

	req := &completeRequest{}
	if err = s.readPayload(doc, req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), CompleteCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), CompleteCommand)
		return
	}

	payload, err := s.makePayload(makeCompleteResponse(proc.Complete(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), CompleteCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), CompleteCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, CompleteCommand)
}

func (s *mdbService) processWait(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), WaitCommand)
		return
	}
	req := waitRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), WaitCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), WaitCommand)
		return
	}

	exitCode, err := proc.Wait(ctx)
	payload, err := s.makePayload(makeWaitResponse(exitCode, err))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), WaitCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), WaitCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, WaitCommand)
}

func (s *mdbService) processRespawn(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), RespawnCommand)
		return
	}
	req := respawnRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), RespawnCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), RespawnCommand)
		return
	}

	pctx, cancel := context.WithCancel(context.Background())
	newProc, err := proc.Respawn(pctx)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("failed to respawned process: %w", err), RespawnCommand)
		cancel()
		return
	}
	if err = s.manager.Register(ctx, newProc); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("failed to register respawned process: %w", err), RespawnCommand)
		cancel()
		return
	}

	if err = newProc.RegisterTrigger(ctx, func(jasper.ProcessInfo) {
		cancel()
	}); err != nil {
		newProcInfo := getProcInfoNoHang(ctx, newProc)
		cancel()
		if !newProcInfo.Complete {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("failed to register trigger on respawned process: %w", err), RespawnCommand)
			return
		}
	}

	payload, err := s.makePayload(makeInfoResponse(newProc.Info(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), RespawnCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), RespawnCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, RespawnCommand)
}

func (s *mdbService) processSignal(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), SignalCommand)
		return
	}
	req := signalRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), SignalCommand)
		return
	}

	id := req.Params.ID
	sig := req.Params.Signal

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), SignalCommand)
		return
	}

	if err := proc.Signal(ctx, syscall.Signal(sig)); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not signal process: %w", err), SignalCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, SignalCommand)
}

func (s *mdbService) processRegisterSignalTriggerID(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), RegisterSignalTriggerIDCommand)
		return
	}

	req := registerSignalTriggerIDRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), RegisterSignalTriggerIDCommand)
		return
	}

	procID := req.Params.ID
	sigID := req.Params.SignalTriggerID

	makeTrigger, ok := jasper.GetSignalTriggerFactory(sigID)
	if !ok {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get signal trigger ID %s", sigID), RegisterSignalTriggerIDCommand)
		return
	}

	proc, err := s.manager.Get(ctx, procID)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), RegisterSignalTriggerIDCommand)
		return
	}

	if err := proc.RegisterSignalTrigger(ctx, makeTrigger()); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not register signal trigger: %w", err), RegisterSignalTriggerIDCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, RegisterSignalTriggerIDCommand)
}

func (s *mdbService) processTag(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), TagCommand)
		return
	}
	req := tagRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), TagCommand)
		return
	}

	id := req.Params.ID
	tag := req.Params.Tag

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), TagCommand)
		return
	}

	proc.Tag(tag)

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, TagCommand)
}

func (s *mdbService) processGetTags(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), GetTagsCommand)
		return
	}

	req := &getTagsRequest{}
	if err = s.readPayload(doc, req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), GetTagsCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), GetTagsCommand)
		return
	}

	payload, err := s.makePayload(makeGetTagsResponse(proc.GetTags()))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), GetTagsCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), GetTagsCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, GetTagsCommand)
}

func (s *mdbService) processResetTags(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), ResetTagsCommand)
		return
	}
	req := resetTagsRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), ResetTagsCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), ResetTagsCommand)
		return
	}

	proc.ResetTags()

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, ResetTagsCommand)
}
