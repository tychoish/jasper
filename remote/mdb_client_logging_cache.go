package remote

import (
	"context"
	"errors"
	"fmt"
	"time"

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
		return nil, fmt.Errorf("could not build request: %w", err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	msg, err := lc.client.doRequest(lc.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed during request: %w", err)
	}

	resp := &loggingCacheCreateAndGetResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return nil, fmt.Errorf("could not parse response document: %w", err)
	}

	if err = resp.SuccessOrError(); err != nil {
		return nil, fmt.Errorf("error in response: %w", err)
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
		return err
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	msg, err := lc.client.doRequest(ctx, req)
	if err != nil {
		return err
	}

	resp := &shell.ErrorResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	return resp.SuccessOrError()
}

func (lc *mdbLoggingCache) Clear(ctx context.Context) error {
	payload, err := lc.client.makeRequest(&loggingCacheClearRequest{Clear: 1})
	if err != nil {
		return err
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	msg, err := lc.client.doRequest(ctx, req)
	if err != nil {
		return err
	}

	resp := &shell.ErrorResponse{}
	if err := lc.client.readRequest(msg, resp); err != nil {
		return fmt.Errorf("could not read response: %w", err)
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
