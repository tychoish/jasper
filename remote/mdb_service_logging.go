package remote

import (
	"context"
	"io"

	"github.com/deciduosity/birch/mrpc/mongowire"
	"github.com/deciduosity/birch/mrpc/shell"
	"github.com/deciduosity/jasper"
	"github.com/pkg/errors"
)

const (
	LoggingSendMessagesCommand        = "send_messages"
	LoggingCacheSizeCommand           = "logging_cache_size"
	LoggingCacheCreateCommand         = "logging_cache_create"
	LoggingCacheDeleteCommand         = "logging_cache_remove"
	LoggingCacheCloseAndRemoveCommand = "logging_cache_close_and_remove"
	LoggingCacheClearCommand          = "logging_cache_clear"
	LoggingCacheGetCommand            = "logging_cache_get"
	LoggingCachePruneCommand          = "logging_cache_prune"
)

func (s *mdbService) loggingSize(ctx context.Context, w io.Writer, msg mongowire.Message) {
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, nil, LoggingCacheSizeCommand)
	if lc == nil {
		return
	}

	s.serviceLoggingCacheResponse(ctx, w, &loggingCacheSizeResponse{Size: lc.Len()}, LoggingCacheSizeCommand)
}

func (s *mdbService) loggingCreate(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &loggingCacheCreateRequest{}
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, req, LoggingCacheCreateCommand)
	if lc == nil {
		return
	}

	cachedLogger, err := lc.Create(req.Params.ID, req.Params.Options)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not create logger"), LoggingCacheCreateCommand)
		return
	}

	s.serviceLoggingCacheResponse(ctx, w, makeLoggingCacheCreateAndGetResponse(cachedLogger), LoggingCacheCreateCommand)
}

func (s *mdbService) loggingGet(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &loggingCacheGetRequest{}
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, req, LoggingCacheGetCommand)
	if lc == nil {
		return
	}

	cachedLogger := lc.Get(req.ID)
	if cachedLogger == nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.New("named logger does not exist"), LoggingCacheGetCommand)
		return
	}

	s.serviceLoggingCacheResponse(ctx, w, makeLoggingCacheCreateAndGetResponse(cachedLogger), LoggingCacheGetCommand)
}

func (s *mdbService) loggingDelete(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &loggingCacheDeleteRequest{}
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, req, LoggingCacheDeleteCommand)
	if lc == nil {
		return
	}

	lc.Remove(req.ID)

	s.serviceLoggingCacheResponse(ctx, w, nil, LoggingCacheDeleteCommand)
}

func (s *mdbService) loggingCloseAndRemove(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &loggingCacheCloseAndRemoveRequest{}
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, req, LoggingCacheCloseAndRemoveCommand)
	if lc == nil {
		return
	}

	if err := lc.CloseAndRemove(ctx, req.ID); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, err, LoggingCacheCloseAndRemoveCommand)
		return
	}

	s.serviceLoggingCacheResponse(ctx, w, nil, LoggingCacheCloseAndRemoveCommand)
}

func (s *mdbService) loggingClear(ctx context.Context, w io.Writer, msg mongowire.Message) {
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, nil, LoggingCacheClearCommand)
	if lc == nil {
		return
	}

	if err := lc.Clear(ctx); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, err, LoggingCacheClearCommand)
		return
	}

	s.serviceLoggingCacheResponse(ctx, w, nil, LoggingCacheClearCommand)
}

func (s *mdbService) loggingPrune(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &loggingCachePruneRequest{}
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, req, LoggingCachePruneCommand)
	if lc == nil {
		return
	}

	lc.Prune(req.LastAccessed)

	s.serviceLoggingCacheResponse(ctx, w, nil, LoggingCachePruneCommand)
}

func (s *mdbService) loggingSendMessages(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &loggingSendMessagesRequest{}
	lc := s.serviceLoggingCacheRequest(ctx, w, msg, req, LoggingSendMessagesCommand)
	if lc == nil {
		return
	}

	if err := req.Payload.Validate(); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "invalid logging payload"), LoggingSendMessagesCommand)
		return
	}

	cachedLogger := lc.Get(req.Payload.LoggerID)
	if cachedLogger == nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.New("named logger does not exist"), LoggingSendMessagesCommand)
		return
	}
	if err := cachedLogger.Send(&req.Payload); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "problem sending message"), LoggingSendMessagesCommand)
		return
	}

	s.serviceLoggingCacheResponse(ctx, w, nil, LoggingSendMessagesCommand)
}

func (s *mdbService) serviceLoggingCacheRequest(ctx context.Context, w io.Writer, msg mongowire.Message, req interface{}, command string) jasper.LoggingCache {
	lc := s.manager.LoggingCache(ctx)
	if lc == nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.New("logging cache not supported"), command)
		return nil
	}

	if req != nil {
		if err := s.readRequest(msg, req); err != nil {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse request"), command)
			return nil
		}
	}

	return lc
}

func (s *mdbService) serviceLoggingCacheResponse(ctx context.Context, w io.Writer, resp interface{}, command string) {
	if resp != nil {
		doc, err := s.makePayload(resp)
		if err != nil {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not parse payload"), command)
			return
		}

		shellResp, err := shell.ResponseToMessage(mongowire.OP_REPLY, doc)
		if err != nil {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrap(err, "could not make response"), command)
			return
		}

		shell.WriteResponse(ctx, w, shellResp, command)
	} else {
		shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, command)
	}
}
