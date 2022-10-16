package remote

import (
	"context"
	"fmt"
	"syscall"

	"github.com/pkg/errors"
	"github.com/tychoish/birch"
	"github.com/tychoish/birch/mrpc/mongowire"
	"github.com/tychoish/birch/mrpc/shell"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

type mdbProcess struct {
	info        jasper.ProcessInfo
	doRequest   func(context.Context, mongowire.Message) (mongowire.Message, error)
	marshaler   options.Marshaler
	unmarshaler options.Unmarshaler
}

func (p *mdbProcess) readRequest(msg mongowire.Message, in interface{}) error {
	doc, err := shell.ResponseMessageToDocument(msg)
	if err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	data, err := doc.MarshalBSON()
	if err != nil {
		return fmt.Errorf("could not read response data: %w", err)
	}

	if err := p.unmarshaler(data, in); err != nil {
		return fmt.Errorf("problem parsing response body: %w", err)

	}

	return nil
}

func (p *mdbProcess) makeRequest(in interface{}) (*birch.Document, error) {
	data, err := p.marshaler(in)
	if err != nil {
		return nil, err
	}

	doc, err := birch.ReadDocument(data)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (p *mdbProcess) ID() string { return p.info.ID }

func (p *mdbProcess) Info(ctx context.Context) jasper.ProcessInfo {
	if p.info.Complete {
		return p.info
	}

	payload, err := p.makeRequest(infoRequest{ID: p.ID()})
	if err != nil {
		grip.Warning(fmt.Errorf("problem marshalling request: %w", err))
		return jasper.ProcessInfo{}
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get process info for process %s", p.ID()))
		return jasper.ProcessInfo{}
	}
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get process info for process %s", p.ID()))
		return jasper.ProcessInfo{}
	}

	resp := &infoResponse{}
	if err := p.readRequest(msg, resp); err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to parse process info for %s", p.ID()))
		return jasper.ProcessInfo{}
	}
	if err := resp.SuccessOrError(); err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get process info for process %s", p.ID()))
		return jasper.ProcessInfo{}
	}
	p.info = resp.Info
	return p.info
}

func (p *mdbProcess) Running(ctx context.Context) bool {
	if p.info.Complete {
		return false
	}

	payload, err := p.makeRequest(runningRequest{ID: p.ID()})
	if err != nil {
		grip.Warning(fmt.Errorf("problem marshalling request: %w", err))
		return false
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get process running status for process %s", p.ID()))
		return false
	}
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get process running status for process %s", p.ID()))
		return false
	}

	var resp runningResponse
	if err := p.readRequest(msg, &resp); err != nil {
		grip.Warning(fmt.Errorf("problem reading response: %w", err))
		return false
	}

	grip.Warning(message.WrapErrorf(resp.SuccessOrError(), "failed to get process running status for process %s", p.ID()))
	return resp.Running
}

func (p *mdbProcess) Complete(ctx context.Context) bool {
	if p.info.Complete {
		return true
	}

	payload, err := p.makeRequest(completeRequest{ID: p.ID()})
	if err != nil {
		grip.Warning(fmt.Errorf("problem marshalling request: %w", err))
		return false
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get process completion status for process %s", p.ID()))
		return false
	}
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get process completion status for process %s", p.ID()))
		return false
	}

	var resp completeResponse
	if err := p.readRequest(msg, &resp); err != nil {
		grip.Warning(fmt.Errorf("problem reading response: %w", err))
		return false
	}

	grip.Warning(message.WrapErrorf(resp.SuccessOrError(), "failed to get process completion status for process %s", p.ID()))
	return resp.Complete
}

func (p *mdbProcess) Signal(ctx context.Context, sig syscall.Signal) error {
	r := signalRequest{}
	r.Params.ID = p.ID()
	r.Params.Signal = int(sig)

	payload, err := p.makeRequest(r)
	if err != nil {
		return fmt.Errorf("problem marshalling request: %w", err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("failed during request: %w", err)
	}

	var resp shell.ErrorResponse
	if err := p.readRequest(msg, &resp); err != nil {
		return fmt.Errorf("problem reading response: %w", err)
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (p *mdbProcess) Wait(ctx context.Context) (int, error) {
	payload, err := p.makeRequest(waitRequest{p.ID()})
	if err != nil {
		return -1, fmt.Errorf("problem marshalling request: %w", err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return -1, fmt.Errorf("could not create request: %w", err)
	}
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		return -1, fmt.Errorf("failed during request: %w", err)
	}

	var resp waitResponse
	if err := p.readRequest(msg, &resp); err != nil {
		return -1, fmt.Errorf("problem reading response: %w", err)
	}

	return resp.ExitCode, errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (p *mdbProcess) Respawn(ctx context.Context) (jasper.Process, error) {
	payload, err := p.makeRequest(respawnRequest{ID: p.ID()})
	if err != nil {
		return nil, fmt.Errorf("problem marshalling request: %w", err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed during request: %w", err)
	}
	var resp infoResponse
	if err := p.readRequest(msg, &resp); err != nil {
		return nil, fmt.Errorf("problem reading response: %w", err)
	}

	return &mdbProcess{info: resp.Info, doRequest: p.doRequest, marshaler: p.marshaler, unmarshaler: p.unmarshaler}, nil
}

func (p *mdbProcess) RegisterTrigger(ctx context.Context, t jasper.ProcessTrigger) error {
	return errors.New("cannot register triggers on remote processes")
}

func (p *mdbProcess) RegisterSignalTrigger(ctx context.Context, t jasper.SignalTrigger) error {
	return errors.New("cannot register signal triggers on remote processes")
}

func (p *mdbProcess) RegisterSignalTriggerID(ctx context.Context, sigID jasper.SignalTriggerID) error {
	r := registerSignalTriggerIDRequest{}
	r.Params.ID = p.ID()
	r.Params.SignalTriggerID = sigID

	payload, err := p.makeRequest(r)
	if err != nil {
		return fmt.Errorf("problem marshalling request: %w", err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("failed during request: %w", err)
	}
	var resp shell.ErrorResponse
	if err := p.readRequest(msg, &resp); err != nil {
		return fmt.Errorf("problem reading response: %w", err)
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (p *mdbProcess) Tag(tag string) {
	r := tagRequest{}
	r.Params.ID = p.ID()
	r.Params.Tag = tag

	payload, err := p.makeRequest(r)
	if err != nil {
		grip.Warning(fmt.Errorf("problem marshalling request: %w", err))
		return
	}
	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warningf("failed to tag process %s with tag %s", p.ID(), tag)
		return
	}
	msg, err := p.doRequest(context.Background(), req)
	if err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to tag process %s with tag %s", p.ID(), tag))
		return
	}
	var resp shell.ErrorResponse
	if err := p.readRequest(msg, &resp); err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to tag process %s", p.ID()))
		return
	}

	grip.Warning(message.WrapErrorf(resp.SuccessOrError(), "failed to tag process %s with tag %s", p.ID(), tag))
}

func (p *mdbProcess) GetTags() []string {
	payload, err := p.makeRequest(getTagsRequest{p.ID()})
	if err != nil {
		grip.Warning(fmt.Errorf("problem marshalling request: %w", err))
		return nil
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warningf("failed to get tags for process %s", p.ID())
		return nil
	}
	msg, err := p.doRequest(context.Background(), req)
	if err != nil {
		grip.Warningf("failed to get tags for process %s", p.ID())
		return nil
	}

	var resp getTagsResponse
	if err = p.readRequest(msg, &resp); err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to get tags for process %s", p.ID()))
		return nil
	}

	return resp.Tags
}

func (p *mdbProcess) ResetTags() {
	payload, err := p.makeRequest(resetTagsRequest{p.ID()})
	if err != nil {
		grip.Warning(fmt.Errorf("problem marshalling request: %w", err))
		return
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warningf("failed to reset tags for process %s", p.ID())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	msg, err := p.doRequest(ctx, req)
	if err != nil {
		grip.Warningf("failed to reset tags for process %s", p.ID())
		return
	}
	var resp shell.ErrorResponse
	if err := p.readRequest(msg, &resp); err != nil {
		grip.Warning(message.WrapErrorf(err, "failed to reset tags for process %s", p.ID()))
		return
	}

	grip.Warning(message.WrapErrorf(resp.SuccessOrError(), "failed to reset tags for process %s", p.ID()))
}
