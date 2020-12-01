package remote

import (
	"context"
	"net"
	"time"

	"github.com/cdr/grip"
	"github.com/cdr/grip/message"
	"github.com/deciduosity/birch"
	"github.com/deciduosity/jasper"
	"github.com/deciduosity/jasper/options"
	"github.com/deciduosity/jasper/scripting"
	"github.com/deciduosity/birch/mrpc/mongowire"
	"github.com/deciduosity/birch/mrpc/shell"
	"github.com/pkg/errors"
)

type mdbClient struct {
	conn        net.Conn
	namespace   string
	timeout     time.Duration
	marshaler   options.Marshaler
	unmarshaler options.Unmarshaler
}

const (
	namespace = "jasper.$cmd"
)

func (c *mdbClient) makeProcess(info jasper.ProcessInfo) *mdbProcess {
	return &mdbProcess{
		info:        info,
		doRequest:   c.doRequest,
		marshaler:   c.marshaler,
		unmarshaler: c.unmarshaler,
	}
}

// NewMDBClient returns a remote client for connection to a MongoDB wire protocol
// service. reqTimeout specifies the timeout for a request, or uses a default
// timeout if zero.
func NewMDBClient(ctx context.Context, addr net.Addr, reqTimeout time.Duration) (Manager, error) {
	client := &mdbClient{
		namespace:   namespace,
		unmarshaler: options.GetGlobalLoggerRegistry().Unmarshaler(options.RawLoggerConfigFormatBSON),
		marshaler:   options.GetGlobalLoggerRegistry().Marshaler(options.RawLoggerConfigFormatBSON),
	}

	client.timeout = reqTimeout
	if client.timeout.Seconds() == 0 {
		client.timeout = 30 * time.Second
	}

	if client.unmarshaler == nil || client.marshaler == nil {
		return nil, errors.New("marshling abilities are not registered")
	}

	dialer := net.Dialer{}
	var err error
	if client.conn, err = dialer.DialContext(ctx, "tcp", addr.String()); err != nil {
		return nil, errors.Wrapf(err, "could not establish connection to %s service at address %s", addr.Network(), addr.String())
	}

	return client, nil
}

func (c *mdbClient) ID() string {
	payload, err := c.makeRequest(&idRequest{ID: 1})
	if err != nil {
		grip.Warning(message.WrapError(err, "could not build request"))
		return ""
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warning(message.WrapError(err, "could not create request"))
		return ""
	}
	msg, err := c.doRequest(context.Background(), req)
	if err != nil {
		grip.Warning(message.WrapError(err, "failed during request"))
		return ""
	}
	resp := &idResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		grip.Warning(message.WrapError(err, "problem reading response"))
		return ""
	}

	if err := resp.SuccessOrError(); err != nil {
		grip.Warning(message.WrapError(err, "error in response"))
		return ""
	}
	return resp.ID
}

func (c *mdbClient) readRequest(msg mongowire.Message, in interface{}) error {
	doc, err := shell.ResponseMessageToDocument(msg)
	if err != nil {
		return errors.Wrap(err, "could not read response")
	}

	data, err := doc.MarshalBSON()
	if err != nil {
		return errors.Wrap(err, "could not read response data")
	}

	if err := c.unmarshaler(data, in); err != nil {
		return errors.Wrap(err, "problem parsing response body")

	}

	return nil
}

func (c *mdbClient) makeRequest(in interface{}) (*birch.Document, error) {
	data, err := c.marshaler(in)
	if err != nil {
		return nil, err
	}

	doc, err := birch.ReadDocument(data)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (c *mdbClient) CreateProcess(ctx context.Context, opts *options.Create) (jasper.Process, error) {
	payload, err := c.makeRequest(&createProcessRequest{Options: *opts})
	if err != nil {
		return nil, errors.Wrap(err, "could not build request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	resp := &infoResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		return nil, errors.Wrap(err, "problem reading response")
	}

	if err := resp.SuccessOrError(); err != nil {
		return nil, errors.Wrap(err, "error in response")
	}

	return c.makeProcess(resp.Info), nil
}

func (c *mdbClient) CreateCommand(ctx context.Context) *jasper.Command {
	return jasper.NewCommand().ProcConstructor(c.CreateProcess)
}

func (c *mdbClient) CreateScripting(ctx context.Context, opts options.ScriptingHarness) (scripting.Harness, error) {
	marshalledOpts, err := c.marshaler(opts)
	if err != nil {
		return nil, errors.Wrap(err, "problem marshalling options")
	}

	r := &scriptingCreateRequest{}
	r.Params.Type = opts.Type()
	r.Params.Options = marshalledOpts

	payload, err := c.makeRequest(r)
	if err != nil {
		return nil, errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}

	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	resp := &scriptingCreateResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		return nil, errors.Wrap(err, "problem reading response")
	}

	if err = resp.SuccessOrError(); err != nil {
		return nil, errors.Wrap(err, "error in response")
	}
	return &mdbScriptingClient{
		client: c,
		id:     resp.ID,
	}, nil
}

func (c *mdbClient) GetScripting(ctx context.Context, id string) (scripting.Harness, error) {
	payload, err := c.makeRequest(&scriptingGetRequest{ID: id})
	if err != nil {
		return nil, errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}

	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		return nil, errors.Wrap(err, "problem reading response")
	}

	if err = resp.SuccessOrError(); err != nil {
		return nil, errors.Wrap(err, "error in response")
	}
	return &mdbScriptingClient{
		client: c,
		id:     id,
	}, nil
}

type mdbScriptingClient struct {
	client *mdbClient
	id     string
}

func (s *mdbScriptingClient) ID() string { return s.id }
func (s *mdbScriptingClient) Setup(ctx context.Context) error {
	payload, err := s.client.makeRequest(&scriptingSetupRequest{ID: s.id})
	if err != nil {
		return errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := s.client.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := s.client.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "problem reading response")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (s *mdbScriptingClient) Cleanup(ctx context.Context) error {
	payload, err := s.client.makeRequest(&scriptingCleanupRequest{ID: s.id})
	if err != nil {
		return errors.Wrap(err, "could not build request")
	}
	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := s.client.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := s.client.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "problem reading response")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (s *mdbScriptingClient) Run(ctx context.Context, args []string) error {
	r := &scriptingRunRequest{}
	r.Params.ID = s.id
	r.Params.Args = args
	payload, err := s.client.makeRequest(r)
	if err != nil {
		return errors.Wrap(err, "could not build request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := s.client.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := s.client.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not parse response document")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (s *mdbScriptingClient) RunScript(ctx context.Context, in string) error {
	r := &scriptingRunScriptRequest{}
	r.Params.ID = s.id
	r.Params.Script = in
	payload, err := s.client.makeRequest(r)
	if err != nil {
		return errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := s.client.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := s.client.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not parse response document")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (s *mdbScriptingClient) Build(ctx context.Context, dir string, args []string) (string, error) {
	r := &scriptingBuildRequest{}
	r.Params.ID = s.id
	r.Params.Dir = dir
	r.Params.Args = args

	payload, err := s.client.makeRequest(r)
	if err != nil {
		return "", errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return "", errors.Wrap(err, "could not create request")
	}

	msg, err := s.client.doRequest(ctx, req)
	if err != nil {
		return "", errors.Wrap(err, "failed during request")
	}

	resp := &scriptingBuildResponse{}
	if err := s.client.readRequest(msg, resp); err != nil {
		return "", errors.Wrap(err, "could not parse response document")
	}

	return resp.Path, errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (s *mdbScriptingClient) Test(ctx context.Context, dir string, opts ...scripting.TestOptions) ([]scripting.TestResult, error) {
	r := &scriptingTestRequest{}
	r.Params.ID = s.id
	r.Params.Dir = dir
	r.Params.Options = opts
	payload, err := s.client.makeRequest(r)
	if err != nil {
		return nil, errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}

	msg, err := s.client.doRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	resp := &scriptingTestResponse{}
	if err := s.client.readRequest(msg, resp); err != nil {
		return nil, errors.Wrap(err, "could not parse response document")
	}

	return resp.Results, errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (c *mdbClient) LoggingCache(ctx context.Context) jasper.LoggingCache {
	return &mdbLoggingCache{
		client: c,
		ctx:    ctx,
	}
}

func (c *mdbClient) SendMessages(ctx context.Context, lp options.LoggingPayload) error {
	payload, err := c.makeRequest(&loggingSendMessageRequest{Payload: lp})
	if err != nil {
		return errors.WithStack(err)
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not parse response document")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

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

func (c *mdbClient) Register(ctx context.Context, proc jasper.Process) error {
	return errors.New("cannot register local processes on remote process managers")
}

func (c *mdbClient) List(ctx context.Context, f options.Filter) ([]jasper.Process, error) {
	payload, err := c.makeRequest(listRequest{Filter: f})
	if err != nil {
		return nil, errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	resp := infosResponse{}
	if err = c.readRequest(msg, &resp); err != nil {
		return nil, errors.Wrap(err, "problem reading response")
	}

	if err := resp.SuccessOrError(); err != nil {
		return nil, errors.Wrap(err, "error in response")
	}
	infos := resp.Infos
	procs := make([]jasper.Process, 0, len(infos))
	for idx := range infos {
		procs = append(procs, c.makeProcess(infos[idx]))
	}
	return procs, nil
}

func (c *mdbClient) Group(ctx context.Context, tag string) ([]jasper.Process, error) {
	payload, err := c.makeRequest(groupRequest{Tag: tag})
	if err != nil {
		return nil, errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	resp := infosResponse{}
	if err = c.readRequest(msg, &resp); err != nil {
		return nil, errors.Wrap(err, "problem reading response")
	}

	if err := resp.SuccessOrError(); err != nil {
		return nil, errors.Wrap(err, "error in response")
	}

	infos := resp.Infos
	procs := make([]jasper.Process, 0, len(infos))
	for idx := range infos {
		procs = append(procs, c.makeProcess(infos[idx]))
	}

	return procs, nil
}

func (c *mdbClient) Get(ctx context.Context, id string) (jasper.Process, error) {
	unmarshaler := options.GetGlobalLoggerRegistry().Unmarshaler(options.RawLoggerConfigFormatBSON)
	if unmarshaler == nil {
		return nil, errors.New("could not find registered unmarshaler")
	}

	payload, err := c.makeRequest(&getProcessRequest{ID: id})
	if err != nil {
		return nil, errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed during request")
	}

	var resp infoResponse
	if err := c.readRequest(msg, &resp); err != nil {
		return nil, errors.Wrap(err, "problem reading response")
	}

	if err := resp.SuccessOrError(); err != nil {
		return nil, errors.Wrap(err, "error in response")
	}

	return c.makeProcess(resp.Info), nil
}

func (c *mdbClient) Clear(ctx context.Context) {
	payload, err := c.makeRequest(&clearRequest{Clear: 1})
	if err != nil {
		grip.Warning(errors.Wrap(err, "problem marshalling request"))
		return
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		grip.Warning(message.WrapError(err, "could not create request"))
		return
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		grip.Warning(message.WrapError(err, "failed during request"))
		return
	}
	resp := &shell.ErrorResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		grip.Warning(errors.Wrap(err, "could not parse response document"))
		return
	}

	grip.Warning(message.WrapError(resp.SuccessOrError(), "error in response"))
}

func (c *mdbClient) Close(ctx context.Context) error {
	payload, err := c.makeRequest(&closeRequest{Close: 1})
	if err != nil {
		return errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}

	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}
	resp := &shell.ErrorResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not parse response document")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (c *mdbClient) WriteFile(ctx context.Context, opts options.WriteFile) error {
	sendOpts := func(opts options.WriteFile) error {
		payload, err := c.makeRequest(writeFileRequest{Options: opts})
		if err != nil {
			return errors.Wrap(err, "could not build request")
		}

		req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
		if err != nil {
			return errors.Wrap(err, "could not create request")
		}
		msg, err := c.doRequest(ctx, req)
		if err != nil {
			return errors.Wrap(err, "failed during request")
		}

		resp := &shell.ErrorResponse{}
		if err := c.readRequest(msg, resp); err != nil {
			return errors.Wrap(err, "could not parse response document")
		}

		return errors.Wrap(resp.SuccessOrError(), "error in response")
	}
	return opts.WriteBufferedContent(sendOpts)
}

// CloseConnection closes the client connection. Callers are expected to call
// this when finished with the client.
func (c *mdbClient) CloseConnection() error {
	return c.conn.Close()
}

func (c *mdbClient) DownloadFile(ctx context.Context, opts options.Download) error {
	payload, err := c.makeRequest(downloadFileRequest{Options: opts})
	if err != nil {
		return errors.Wrap(err, "could not build request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not parse response document")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

func (c *mdbClient) GetLogStream(ctx context.Context, id string, count int) (jasper.LogStream, error) {
	r := getLogStreamRequest{}
	r.Params.ID = id
	r.Params.Count = count
	data, err := c.marshaler(r)
	if err != nil {
		return jasper.LogStream{}, errors.Wrap(err, "could not marshal request")
	}
	payload, err := birch.ReadDocument(data)
	if err != nil {
		return jasper.LogStream{}, errors.Wrap(err, "could construct request payload")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return jasper.LogStream{}, errors.Wrap(err, "could not create request")
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return jasper.LogStream{}, errors.Wrap(err, "failed during request")
	}

	var resp getLogStreamResponse
	if err := c.readRequest(msg, &resp); err != nil {
		return jasper.LogStream{}, errors.Wrap(err, "problem reading response")
	}

	if err := resp.SuccessOrError(); err != nil {
		return jasper.LogStream{}, errors.Wrap(err, "error in response")
	}

	return resp.LogStream, nil
}

func (c *mdbClient) SignalEvent(ctx context.Context, name string) error {
	payload, err := c.makeRequest(signalEventRequest{Name: name})
	if err != nil {
		return errors.Wrap(err, "problem marshalling request")
	}

	req, err := shell.RequestToMessage(mongowire.OP_QUERY, payload)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	msg, err := c.doRequest(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed during request")
	}

	resp := &shell.ErrorResponse{}
	if err := c.readRequest(msg, resp); err != nil {
		return errors.Wrap(err, "could not parse response document")
	}

	return errors.Wrap(resp.SuccessOrError(), "error in response")
}

// doRequest sends the given request and reads the response.
func (c *mdbClient) doRequest(ctx context.Context, req mongowire.Message) (mongowire.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := mongowire.SendMessage(ctx, req, c.conn); err != nil {
		return nil, errors.Wrap(err, "problem sending request")
	}
	msg, err := mongowire.ReadMessage(ctx, c.conn)
	if err != nil {
		return nil, errors.Wrap(err, "error in response")
	}
	return msg, nil
}
