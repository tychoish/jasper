package remote

import (
	"context"
	"io"
	"syscall"

	"github.com/deciduosity/jasper"
	"github.com/tychoish/birch/mrpc/mongowire"
	"github.com/tychoish/birch/mrpc/shell"
	"github.com/pkg/errors"
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
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), InfoCommand)
		return
	}
	req := infoRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), InfoCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), InfoCommand)
		return
	}

	payload, err := s.makePayload(makeInfoResponse(proc.Info(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), InfoCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), InfoCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, InfoCommand)
}

func (s *mdbService) processRunning(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), RunningCommand)
		return
	}
	req := runningRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), RunningCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), RunningCommand)
		return
	}

	payload, err := s.makePayload(makeRunningResponse(proc.Running(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), RunningCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), RunningCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, RunningCommand)
}

func (s *mdbService) processComplete(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), CompleteCommand)
		return
	}

	req := &completeRequest{}
	if err = s.readPayload(doc, req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), CompleteCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), CompleteCommand)
		return
	}

	payload, err := s.makePayload(makeCompleteResponse(proc.Complete(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), CompleteCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), CompleteCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, CompleteCommand)
}

func (s *mdbService) processWait(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), WaitCommand)
		return
	}
	req := waitRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), WaitCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), WaitCommand)
		return
	}

	exitCode, err := proc.Wait(ctx)
	payload, err := s.makePayload(makeWaitResponse(exitCode, err))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), WaitCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), WaitCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, WaitCommand)
}

func (s *mdbService) processRespawn(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), RespawnCommand)
		return
	}
	req := respawnRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), RespawnCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), RespawnCommand)
		return
	}

	pctx, cancel := context.WithCancel(context.Background())
	newProc, err := proc.Respawn(pctx)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "failed to respawned process"), RespawnCommand)
		cancel()
		return
	}
	if err = s.manager.Register(ctx, newProc); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "failed to register respawned process"), RespawnCommand)
		cancel()
		return
	}

	if err = newProc.RegisterTrigger(ctx, func(jasper.ProcessInfo) {
		cancel()
	}); err != nil {
		newProcInfo := getProcInfoNoHang(ctx, newProc)
		cancel()
		if !newProcInfo.Complete {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "failed to register trigger on respawned process"), RespawnCommand)
			return
		}
	}

	payload, err := s.makePayload(makeInfoResponse(newProc.Info(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), RespawnCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), RespawnCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, RespawnCommand)
}

func (s *mdbService) processSignal(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), SignalCommand)
		return
	}
	req := signalRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), SignalCommand)
		return
	}

	id := req.Params.ID
	sig := req.Params.Signal

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), SignalCommand)
		return
	}

	if err := proc.Signal(ctx, syscall.Signal(sig)); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not signal process"), SignalCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, SignalCommand)
}

func (s *mdbService) processRegisterSignalTriggerID(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), RegisterSignalTriggerIDCommand)
		return
	}

	req := registerSignalTriggerIDRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), RegisterSignalTriggerIDCommand)
		return
	}

	procID := req.Params.ID
	sigID := req.Params.SignalTriggerID

	makeTrigger, ok := jasper.GetSignalTriggerFactory(sigID)
	if !ok {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Errorf("could not get signal trigger ID %s", sigID), RegisterSignalTriggerIDCommand)
		return
	}

	proc, err := s.manager.Get(ctx, procID)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), RegisterSignalTriggerIDCommand)
		return
	}

	if err := proc.RegisterSignalTrigger(ctx, makeTrigger()); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not register signal trigger"), RegisterSignalTriggerIDCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, RegisterSignalTriggerIDCommand)
}

func (s *mdbService) processTag(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), TagCommand)
		return
	}
	req := tagRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), TagCommand)
		return
	}

	id := req.Params.ID
	tag := req.Params.Tag

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), TagCommand)
		return
	}

	proc.Tag(tag)

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, TagCommand)
}

func (s *mdbService) processGetTags(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), GetTagsCommand)
		return
	}

	req := &getTagsRequest{}
	if err = s.readPayload(doc, req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), GetTagsCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), GetTagsCommand)
		return
	}

	payload, err := s.makePayload(makeGetTagsResponse(proc.GetTags()))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), GetTagsCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), GetTagsCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, GetTagsCommand)
}

func (s *mdbService) processResetTags(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), ResetTagsCommand)
		return
	}
	req := resetTagsRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), ResetTagsCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), ResetTagsCommand)
		return
	}

	proc.ResetTags()

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, ResetTagsCommand)
}
