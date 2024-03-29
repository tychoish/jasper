package remote

import (
	"context"
	"errors"
	"fmt"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/tychoish/jasper/options"
	internal "github.com/tychoish/jasper/x/remote/internal"
)

// rpcLoggingCache is the client-side representation of a jasper.LoggingCache
// for making requests to the remote gRPC service.
type rpcLoggingCache struct {
	client internal.JasperProcessManagerClient
	ctx    context.Context
}

func (lc *rpcLoggingCache) Create(id string, opts *options.Output) (*options.CachedLogger, error) {
	args, err := internal.ConvertLoggingCreateArgs(id, opts)
	if err != nil {
		return nil, fmt.Errorf("problem converting create args: %w", err)
	}
	resp, err := lc.client.LoggingCacheCreate(lc.ctx, args)
	if err != nil {
		return nil, err
	}

	out, err := resp.Export()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (lc *rpcLoggingCache) Put(id string, opts *options.CachedLogger) error {
	return errors.New("operation not supported for remote managers")
}

func (lc *rpcLoggingCache) Get(id string) *options.CachedLogger {
	resp, err := lc.client.LoggingCacheGet(lc.ctx, &internal.LoggingCacheArgs{Name: id})
	if err != nil {
		return nil
	}
	if !resp.Outcome.Success {
		return nil
	}

	out, err := resp.Export()
	if err != nil {
		return nil
	}

	return out
}

func (lc *rpcLoggingCache) Remove(id string) {
	_, _ = lc.client.LoggingCacheRemove(lc.ctx, &internal.LoggingCacheArgs{Name: id})
}

func (lc *rpcLoggingCache) CloseAndRemove(ctx context.Context, id string) error {
	resp, err := lc.client.LoggingCacheCloseAndRemove(ctx, &internal.LoggingCacheArgs{Name: id})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("failed to close and remove: %s", resp.Text)
	}
	return nil
}

func (lc *rpcLoggingCache) Clear(ctx context.Context) error {
	resp, err := lc.client.LoggingCacheClear(ctx, &empty.Empty{})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("failed to clear the logging cache: %s", resp.Text)
	}
	return nil
}

func (lc *rpcLoggingCache) Prune(ts time.Time) {
	_, _ = lc.client.LoggingCachePrune(lc.ctx, timestamppb.New(ts))
}

func (lc *rpcLoggingCache) Len() int {
	resp, err := lc.client.LoggingCacheLen(lc.ctx, &empty.Empty{})
	if err != nil {
		return -1
	}

	return int(resp.Size)
}
