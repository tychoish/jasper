package remote

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/tychoish/birch"
	"github.com/tychoish/birch/mrpc/mongowire"
	"github.com/tychoish/birch/mrpc/shell"
	"github.com/tychoish/jasper"
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
	payload, err := s.makePayload(makeIDResponse(s.manager.ID()))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.New("could not build response"), ManagerIDCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.New("could not make response"), ManagerIDCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, ManagerIDCommand)
}

func (s *mdbService) managerCreateProcess(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), CreateProcessCommand)
		return
	}

	req := createProcessRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request body: %w", err), CreateProcessCommand)
		return
	}

	opts := req.Options

	// Spawn a new context so that the process' context is not potentially
	// canceled by the request's. See how rest_service.go's createProcess() does
	// this same thing.
	pctx, cancel := context.WithCancel(context.Background())

	proc, err := s.manager.CreateProcess(pctx, &opts)
	if err != nil {
		cancel()
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not create process: %w", err), CreateProcessCommand)
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
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not register trigger: %w", err), CreateProcessCommand)
			return
		}
	}

	payload, err := s.makePayload(makeInfoResponse(getProcInfoNoHang(ctx, proc)))
	if err != nil {
		cancel()
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem building response: %w", err), CreateProcessCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		cancel()
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), CreateProcessCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, CreateProcessCommand)
}

func (s *mdbService) readPayload(doc birch.Marshaler, in interface{}) error {
	data, err := doc.MarshalBSON()
	if err != nil {
		return fmt.Errorf("problem reading payload: %w", err)
	}

	return errors.Wrap(s.unmarshaler(data, in), "problem parsing document")
}

func (s *mdbService) managerList(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), ListCommand)
		return
	}
	req := listRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), ListCommand)
		return
	}

	filter := req.Filter

	procs, err := s.manager.List(ctx, filter)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not list processes: %w", err), ListCommand)
		return
	}

	infos := make([]jasper.ProcessInfo, 0, len(procs))
	for _, proc := range procs {
		infos = append(infos, proc.Info(ctx))
	}

	payload, err := s.makePayload(makeInfosResponse(infos))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem building response: %w", err), ListCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), ListCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, ListCommand)
}

func (s *mdbService) managerGroup(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), GroupCommand)
		return
	}

	req := groupRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), GroupCommand)
		return
	}

	tag := req.Tag

	procs, err := s.manager.Group(ctx, tag)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process group: %w", err), GroupCommand)
		return
	}

	infos := make([]jasper.ProcessInfo, 0, len(procs))
	for _, proc := range procs {
		infos = append(infos, proc.Info(ctx))
	}

	payload, err := s.makePayload(makeInfosResponse(infos))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), GroupCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), GroupCommand)
		return
	}
	shell.WriteResponse(ctx, w, resp, GroupCommand)
}

func (s *mdbService) managerGetProcess(ctx context.Context, w io.Writer, msg mongowire.Message) {
	doc, err := shell.RequestMessageToDocument(msg)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), GetProcessCommand)
		return
	}

	req := getProcessRequest{}
	if err = s.readPayload(doc, &req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), GetProcessCommand)
		return
	}

	id := req.ID

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not get process: %w", err), GetProcessCommand)
		return
	}

	payload, err := s.makePayload(makeInfoResponse(proc.Info(ctx)))
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not build response: %w", err), GetProcessCommand)
		return
	}

	resp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), GetProcessCommand)
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
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), WriteFileCommand)
		return
	}

	req := &writeFileRequest{}
	if err = s.readPayload(doc, req); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), WriteFileCommand)
		return
	}

	opts := req.Options

	if err := opts.Validate(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("invalid write file options: %w", err), WriteFileCommand)
		return
	}
	if err := opts.DoWrite(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("failed to write to file: %w", err), WriteFileCommand)
		return
	}

	shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, WriteFileCommand)
}
