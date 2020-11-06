package options

import (
	"context"

	"cdr.dev/wsep"
	"github.com/deciduosity/jasper/util"
	"github.com/deciduosity/utility"
	"github.com/pkg/errors"
	"nhooyr.io/websocket"
)

type WebSocketExec struct {
	URL         string
	DialOptions *websocket.DialOptions
	Connection  *websocket.Conn
}

func (opts *WebSocketExec) Validate() error {
	if opts.Connection != nil {
		return nil
	}

	if opts.URL == "" {
		return errors.New("must specify a ")

	}

	return errors.New("not implemented")

}

func (opts *WebSocketExec) Resolve(ctx context.Context) (wsep.Execer, util.CloseFunc, error) {
	noopCloser := func() error { return nil }
	var closer = noopCloser

	if opts.Connection != nil {
		return wsep.RemoteExecer(opts.Connection), noopCloser, nil
	}

	if opts.DialOptions == nil {
		opts.DialOptions = &websocket.DialOptions{}
	}

	if opts.DialOptions.HTTPClient == nil {
		opts.DialOptions.HTTPClient = utility.GetHTTPClient()
		closer = func() error {
			utility.PutHTTPClient(opts.DialOptions.HTTPClient)
			return nil
		}
	}

	conn, resp, err := websocket.Dial(ctx, opts.URL, opts.DialOptions)
	if err != nil {
		closer()
		return nil, noopCloser, errors.Wrap(err, "problem dialing connection")
	}
	if resp.StatusCode >= 400 {
		closer()
		return nil, noopCloser, errors.New("http connection error")
	}

	opts.Connection = conn
	return wsep.RemoteExecer(opts.Connection), closer, nil
}
