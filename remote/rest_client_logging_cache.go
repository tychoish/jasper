package remote

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/gimlet"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper/options"
)

// restLoggingCache is the client-side representation of a jasper.LoggingCache
// for making requests to the remote REST service.
type restLoggingCache struct {
	client *restClient
	ctx    context.Context
}

func (lc *restLoggingCache) Create(id string, opts *options.Output) (*options.CachedLogger, error) {
	body, err := makeBody(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := lc.client.doRequest(lc.ctx, http.MethodPost, lc.client.getURL("/logging/id/%s", id), body)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close()

	if err = handleError(resp); err != nil {
		return nil, errors.WithStack(err)
	}

	out := &options.CachedLogger{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil, errors.WithStack(err)
	}

	return out, nil
}

func (lc *restLoggingCache) Put(id string, cl *options.CachedLogger) error {
	return errors.New("operation not supported for remote managers")
}

func (lc *restLoggingCache) Get(id string) *options.CachedLogger {
	resp, err := lc.client.doRequest(lc.ctx, http.MethodGet, lc.client.getURL("/logging/id/%s", id), nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if err = handleError(resp); err != nil {
		return nil
	}

	out := &options.CachedLogger{}
	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return nil
	}

	if out.ID == "" {
		return nil
	}

	return out
}

func (lc *restLoggingCache) Remove(id string) {
	resp, err := lc.client.doRequest(lc.ctx, http.MethodDelete, lc.client.getURL("/logging/id/%s", id), nil)
	grip.Info(message.Fields{
		"has_error": err == nil,
		"code":      resp.StatusCode,
		"status":    resp.Status,
		"op":        "delete",
		"logger":    id,
		"err":       err,
	})
}

func (lc *restLoggingCache) Prune(ts time.Time) {
	resp, err := lc.client.doRequest(lc.ctx, http.MethodDelete, lc.client.getURL("/logging/prune/%s", ts.Format(time.RFC3339)), nil)
	grip.Info(message.Fields{
		"has_error": err == nil,
		"code":      resp.StatusCode,
		"status":    resp.Status,
		"op":        "prune",
		"err":       err,
	})
}

func (lc *restLoggingCache) Len() int {
	resp, err := lc.client.doRequest(lc.ctx, http.MethodDelete, lc.client.getURL("/logging/size"), nil)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0
	}

	out := restLoggingCacheSize{}
	if err = gimlet.GetJSON(resp.Body, &out); err != nil {
		return 0
	}

	return out.Size
}

func (lc *restLoggingCache) CloseAndRemove(ctx context.Context, id string) error {
	resp, err := lc.client.doRequest(ctx, http.MethodDelete, lc.client.getURL("/logging/id/%s/close", id), nil)
	if err != nil {
		return errors.Wrap(err, "request returned error")
	}
	defer resp.Body.Close()

	return errors.WithStack(handleError(resp))
}

func (lc *restLoggingCache) Clear(ctx context.Context) error {
	resp, err := lc.client.doRequest(ctx, http.MethodDelete, lc.client.getURL("/logging/clear"), nil)
	if err != nil {
		return errors.Wrap(err, "request returned error")
	}
	defer resp.Body.Close()

	return errors.WithStack(handleError(resp))
}
