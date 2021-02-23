package remote

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/birch/mrpc/mongowire"
	"github.com/tychoish/birch/mrpc/shell"
	"github.com/tychoish/jasper/options"
)

type mdbLoggingCache struct {
	client *mdbClient
	ctx    context.Context
}

func (lc *mdbLoggingCache) Create(id string, opts *options.Output) (*options.CachedLogger, error) {
	r := &loggingCacheCreateRequest{}
	r.Params.ID = id
	r.Params.Options = opts
	payload, err := lc.client.makeRequest(r)
	if err != nil {
		return nil, errors.Wrap(err, "could not build request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}

	msg, err := lc.client.doRequest(lc.ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	resp := &loggingCacheCreateAndGetResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return nil, errors.Wrap(err, "could not parse response document")
	}

	if err = resp.SuccessOrError(); err != nil {
		return nil, errors.Wrap(err, "error in response")
	}

	return resp.CachedLogger, nil
}

func (lc *mdbLoggingCache) Put(_ string, _ *options.CachedLogger) error {
	return errors.New("operation not supported for remote managers")
}

func (lc *mdbLoggingCache) Get(id string) *options.CachedLogger {
	payload, err := lc.client.makeRequest(&loggingCacheGetRequest{ID: id})
	if err != nil {
		return nil
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil
	}

	msg, err := lc.client.doRequest(lc.ctx, req)
	if err != nil {
		return nil
	}

	resp := &loggingCacheCreateAndGetResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return nil
	}

	if err = resp.SuccessOrError(); err != nil {
		return nil
	}

	return resp.CachedLogger
}

func (lc *mdbLoggingCache) Remove(id string) {
	payload, err := lc.client.makeRequest(&loggingCacheDeleteRequest{ID: id})
	if err != nil {
		return
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return
	}

	_, _ = lc.client.doRequest(lc.ctx, req)
}

func (lc *mdbLoggingCache) CloseAndRemove(ctx context.Context, id string) error {
	payload, err := lc.client.makeRequest(&loggingCacheCloseAndRemoveRequest{ID: id})
	if err != nil {
		return errors.WithStack(err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := lc.client.doRequest(ctx, req)
	if err != nil {
		return err
	}

	resp := &shell.ErrorResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not read response")
	}

	return resp.SuccessOrError()
}

func (lc *mdbLoggingCache) Clear(ctx context.Context) error {
	payload, err := lc.client.makeRequest(&loggingCacheClearRequest{Clear: 1})
	if err != nil {
		return errors.WithStack(err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := lc.client.doRequest(ctx, req)
	if err != nil {
		return err
	}

	resp := &shell.ErrorResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not read response")
	}

	return resp.SuccessOrError()
}

func (lc *mdbLoggingCache) Prune(lastAccessed time.Time) {
	payload, err := lc.client.makeRequest(&loggingCachePruneRequest{LastAccessed: lastAccessed})
	if err != nil {
		return
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return
	}

	_, _ = lc.client.doRequest(lc.ctx, req)
}

func (lc *mdbLoggingCache) Len() int {
	payload, err := lc.client.makeRequest(&loggingCacheLenRequest{})
	if err != nil {
		return -1
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return -1
	}

	msg, err := lc.client.doRequest(lc.ctx, req)
	if err != nil {
		return -1
	}

	resp := &loggingCacheSizeResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return -1
	}

	return resp.Size
}
