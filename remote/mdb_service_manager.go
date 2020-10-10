package remote

import (
	"context"
	"io"

	"github.com/deciduosity/birch"
	"github.com/deciduosity/jasper"
	"github.com/deciduosity/mrpc/mongowire"
	"github.com/deciduosity/mrpc/shell"
	"github.com/pkg/errors"
)

// Constants representing manager commands.
const (
	ManagerIDCommand     = "id"
	CreateProcessCommand = "create_process"
	GetProcessCommand    = "get_process"
	ListCommand          = "list"
	GroupCommand         = "group"
	ClearCommand         = "clear"
	CloseCommand         = "close"
	WriteFileCommand     = "write_file"
)

func (s *mdbService) managerID(ctx context.Context, w io.Writer, msg mongowire.Message) {
	payload, _ := makeIDResponse(s.manager.ID()).MarshalDocument()
	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.New("could not make response"), ManagerIDCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, ManagerIDCommand)
}

func (s *mdbService) managerCreateProcess(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := createProcessRequest{}
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), CreateProcessCommand)
		return
	}
	data, err := doc.MarshalBSON()
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request body"), CreateProcessCommand)
		return
	}

	if err = s.unmarshaler(data, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse body"), CreateProcessCommand)
	}

	opts := req.Options

	// Spawn a new context so that the process' context is not potentially
	// canceled by the request's. See how rest_service.go's createProcess() does
	// this same thing.
	pctx, cancel := context.WithCancel(context.Background())

	proc, err := s.manager.CreateProcess(pctx, &opts)
	if err != nil {
		cancel()
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not create process"), CreateProcessCommand)
		return
	}

	if err = proc.RegisterTrigger(ctx, func(_ jasper.ProcessInfo) {
		cancel()
	}); err != nil {
		info := getProcInfoNoHang(ctx, proc)
		cancel()
		// If we get an error registering a trigger, then we should make sure that
		// the reason for it isn't just because the process has exited already,
		// since that should not be considered an error.
		if !info.Complete {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not register trigger"), CreateProcessCommand)
			return
		}
	}

	payload, err := s.makePayload(makeInfoResponse(getProcInfoNoHang(ctx, proc)))
	if err != nil {
		cancel()
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "problem building response"), CreateProcessCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		cancel()
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), CreateProcessCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, CreateProcessCommand)
}

func (s *mdbService) readPayload(doc *birch.Document, in interface{}) error {
	data, err := doc.MarshalBSON()
	if err != nil {
		return errors.Wrap(err, "problem reading payload")
	}

	return errors.Wrap(s.unmarshaler(data, in), "problem parsing document")
}

func (s *mdbService) managerList(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), ListCommand)
		return
	}
	req := listRequest{}
	if s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), ListCommand)
		return
	}

	filter := req.Filter

	procs, err := s.manager.List(ctx, filter)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not list processes"), ListCommand)
		return
	}

	infos := make([]jasper.ProcessInfo, 0, len(procs))
	for _, proc := range procs {
		infos = append(infos, proc.Info(ctx))
	}

	payload, err := s.makePayload(makeInfosResponse(infos))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "problem building response"), ListCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), ListCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, ListCommand)
}

func (s *mdbService) managerGroup(ctx context.Context, w io.Writer, msg mongowire.Message) {

	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), GroupCommand)
		return
	}

	req := groupRequest{}
	if s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), GroupCommand)
		return
	}

	tag := req.Tag

	procs, err := s.manager.Group(ctx, tag)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process group"), GroupCommand)
		return
	}

	infos := make([]jasper.ProcessInfo, 0, len(procs))
	for _, proc := range procs {
		infos = append(infos, proc.Info(ctx))
	}

	payload, err := s.makePayload(makeInfosResponse(infos))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), GroupCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), GroupCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, GroupCommand)
}

func (s *mdbService) managerGetProcess(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), GetProcessCommand)
		return
	}

	req := getProcessRequest{}
	if s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), GetProcessCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not get process"), GetProcessCommand)
		return
	}

	payload, err := s.makePayload(makeInfoResponse(proc.Info(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not build response"), GetProcessCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), GetProcessCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, GetProcessCommand)
}

func (s *mdbService) managerClear(ctx context.Context, w io.Writer, msg mongowire.Message) {
	s.manager.Clear(ctx)
	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, ClearCommand)
}

func (s *mdbService) managerClose(ctx context.Context, w io.Writer, msg mongowire.Message) {
	if err := s.manager.Close(ctx); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, err, CloseCommand)
		return
	}
	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, CloseCommand)
}

func (s *mdbService) managerWriteFile(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not read request"), WriteFileCommand)
		return
	}

	req := &writeFileRequest{}
	if s.readPayload(doc, req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), WriteFileCommand)
		return
	}

	opts := req.Options

	if err := opts.Validate(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "invalid write file options"), WriteFileCommand)
		return
	}
	if err := opts.DoWrite(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "failed to write to file"), WriteFileCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, WriteFileCommand)
}
