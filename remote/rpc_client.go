package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"syscall"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/tychoish/emt"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	internal "github.com/tychoish/jasper/remote/internal"
	"github.com/tychoish/jasper/scripting"
	"github.com/tychoish/jasper/util"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type rpcClient struct {
	client       internal.JasperProcessManagerClient
	clientCloser util.CloseFunc
}

// NewClient creates a connection to the RPC service with the specified address
// addr. If creds is non-nil, the credentials will be used to establish a secure
// TLS connection with the service; otherwise, it will establish an insecure
// connection. The caller is responsible for closing the connection using the
// returned jasper.CloseFunc.
func NewRPCClient(ctx context.Context, addr net.Addr, creds *options.CertificateCredentials) (Manager, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	}
	if creds != nil {
		tlsConf, err := creds.Resolve()
		if err != nil {
			return nil, fmt.Errorf("could not resolve credentials into TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConf)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.DialContext(ctx, addr.String(), opts...)
	if err != nil {
		return nil, errors.Wrapf(err, "could not establish connection to %s service at address %s", addr.Network(), addr.String())
	}

	return newRPCClient(conn), nil
}

// NewClientWithFile is the same as NewClient but the credentials will
// be read from the file given by filePath if the filePath is non-empty. The
// credentials file should contain the JSON-encoded bytes from
// (*certdepot.Credentials).Export().
func NewRPCClientWithFile(ctx context.Context, addr net.Addr, filePath string) (Manager, error) {
	var creds *options.CertificateCredentials
	if filePath != "" {
		var err error
		creds, err = options.NewCredentialsFromFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error getting credentials from file: %w", err)
		}
	}

	return NewRPCClient(ctx, addr, creds)
}

// newRPCClient is a constructor for an RPC client.
func newRPCClient(cc *grpc.ClientConn) Manager {
	return &rpcClient{
		client:       internal.NewJasperProcessManagerClient(cc),
		clientCloser: cc.Close,
	}
}

func (c *rpcClient) ID() string {
	resp, err := c.client.ID(context.Background(), &empty.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *rpcClient) CreateProcess(ctx context.Context, opts *options.Create) (jasper.Process, error) {
	convertedOpts, err := internal.ConvertCreateOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("problem converting create options: %w", err)
	}
	proc, err := c.client.Create(ctx, convertedOpts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &rpcProcess{client: c.client, info: proc}, nil
}

func (c *rpcClient) CreateCommand(ctx context.Context) *jasper.Command {
	return jasper.NewCommand().ProcConstructor(c.CreateProcess)
}

func (c *rpcClient) CreateScripting(ctx context.Context, opts options.ScriptingHarness) (scripting.Harness, error) {
	seOpts, err := internal.ConvertScriptingOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("invalid scripting options: %w", err)
	}
	seid, err := c.client.ScriptingHarnessCreate(ctx, seOpts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &rpcScripting{client: c.client, id: seid.Id}, nil
}

func (c *rpcClient) GetScripting(ctx context.Context, id string) (scripting.Harness, error) {
	resp, err := c.client.ScriptingHarnessCheck(ctx, &internal.ScriptingHarnessID{Id: id})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !resp.Success {
		return nil, errors.New(resp.Text)
	}

	return &rpcScripting{client: c.client, id: id}, nil
}

func (c *rpcClient) Register(ctx context.Context, proc jasper.Process) error {
	return errors.New("cannot register local processes on remote process managers")
}

func (c *rpcClient) List(ctx context.Context, f options.Filter) ([]jasper.Process, error) {
	procs, err := c.client.List(ctx, internal.ConvertFilter(f))
	if err != nil {
		return nil, fmt.Errorf("problem getting streaming client: %w", err)
	}

	out := []jasper.Process{}
	for {
		info, err := procs.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("problem getting list: %w", err)
		}

		out = append(out, &rpcProcess{
			client: c.client,
			info:   info,
		})
	}

	return out, nil
}

func (c *rpcClient) Group(ctx context.Context, name string) ([]jasper.Process, error) {
	procs, err := c.client.Group(ctx, &internal.TagName{Value: name})
	if err != nil {
		return nil, fmt.Errorf("problem getting streaming client: %w", err)
	}

	out := []jasper.Process{}
	for {
		info, err := procs.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("problem getting group: %w", err)
		}

		out = append(out, &rpcProcess{
			client: c.client,
			info:   info,
		})
	}

	return out, nil
}

func (c *rpcClient) Get(ctx context.Context, name string) (jasper.Process, error) {
	info, err := c.client.Get(ctx, &internal.JasperProcessID{Value: name})
	if err != nil {
		return nil, fmt.Errorf("problem finding process: %w", err)
	}

	return &rpcProcess{client: c.client, info: info}, nil
}

func (c *rpcClient) Clear(ctx context.Context) {
	_, _ = c.client.Clear(ctx, &empty.Empty{})
}

func (c *rpcClient) Close(ctx context.Context) error {
	resp, err := c.client.Close(ctx, &empty.Empty{})
	if err != nil {
		return errors.WithStack(err)
	}
	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (c *rpcClient) Status(ctx context.Context) (string, bool, error) {
	resp, err := c.client.Status(ctx, &empty.Empty{})
	if err != nil {
		return "", false, errors.WithStack(err)
	}
	return resp.HostId, resp.Active, nil
}

func (c *rpcClient) CloseConnection() error {
	return c.clientCloser()
}

func (c *rpcClient) DownloadFile(ctx context.Context, opts options.Download) error {
	resp, err := c.client.DownloadFile(ctx, internal.ConvertDownloadOptions(opts))
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (c *rpcClient) GetLogStream(ctx context.Context, id string, count int) (jasper.LogStream, error) {
	stream, err := c.client.GetLogStream(ctx, &internal.LogRequest{
		Id:    &internal.JasperProcessID{Value: id},
		Count: int64(count),
	})
	if err != nil {
		return jasper.LogStream{}, errors.WithStack(err)
	}
	return stream.Export(), nil
}

func (c *rpcClient) SignalEvent(ctx context.Context, name string) error {
	resp, err := c.client.SignalEvent(ctx, &internal.EventName{Value: name})
	if err != nil {
		return errors.WithStack(err)
	}
	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (c *rpcClient) WriteFile(ctx context.Context, jopts options.WriteFile) error {
	stream, err := c.client.WriteFile(ctx)
	if err != nil {
		return fmt.Errorf("error getting client stream to write file: %w", err)
	}

	sendOpts := func(jopts options.WriteFile) error {
		opts := internal.ConvertWriteFileOptions(jopts)
		return stream.Send(opts)
	}

	if err = jopts.WriteBufferedContent(sendOpts); err != nil {
		catcher := emt.NewBasicCatcher()
		catcher.Add(err)
		catcher.Add(stream.CloseSend())
		return catcher.Resolve()
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (c *rpcClient) SendMessages(ctx context.Context, lp options.LoggingPayload) error {
	resp, err := c.client.SendMessages(ctx, internal.ConvertLoggingPayload(lp))
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (c *rpcClient) LoggingCache(ctx context.Context) jasper.LoggingCache {
	return &rpcLoggingCache{ctx: ctx, client: c.client}
}

type rpcProcess struct {
	client internal.JasperProcessManagerClient
	info   *internal.ProcessInfo
}

func (p *rpcProcess) ID() string { return p.info.Id }

func (p *rpcProcess) Info(ctx context.Context) jasper.ProcessInfo {
	if p.info.Complete {
		exportedInfo, err := p.info.Export()
		grip.Warning(message.WrapError(err, message.Fields{
			"message": "could not convert info for process",
			"process": p.ID(),
		}))
		return exportedInfo
	}

	info, err := p.client.Get(ctx, &internal.JasperProcessID{Value: p.info.Id})
	if err != nil {
		return jasper.ProcessInfo{}
	}
	p.info = info

	exportedInfo, err := p.info.Export()
	grip.Warning(message.WrapError(err, message.Fields{
		"message": "could not convert info for process",
		"process": p.ID(),
	}))

	return exportedInfo
}
func (p *rpcProcess) Running(ctx context.Context) bool {
	if p.info.Complete {
		return false
	}

	info, err := p.client.Get(ctx, &internal.JasperProcessID{Value: p.info.Id})
	if err != nil {
		return false
	}
	p.info = info

	return info.Running
}

func (p *rpcProcess) Complete(ctx context.Context) bool {
	if p.info.Complete {
		return true
	}

	info, err := p.client.Get(ctx, &internal.JasperProcessID{Value: p.info.Id})
	if err != nil {
		return false
	}
	p.info = info

	return info.Complete
}

func (p *rpcProcess) Signal(ctx context.Context, sig syscall.Signal) error {
	resp, err := p.client.Signal(ctx, &internal.SignalProcess{
		ProcessID: &internal.JasperProcessID{Value: p.info.Id},
		Signal:    internal.ConvertSignal(sig),
	})

	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (p *rpcProcess) Wait(ctx context.Context) (int, error) {
	resp, err := p.client.Wait(ctx, &internal.JasperProcessID{Value: p.info.Id})
	if err != nil {
		return -1, errors.WithStack(err)
	}

	if !resp.Success {
		return int(resp.ExitCode), errors.Wrapf(errors.New(resp.Text), "process exited with error")
	}

	return int(resp.ExitCode), nil
}

func (p *rpcProcess) Respawn(ctx context.Context) (jasper.Process, error) {
	newProc, err := p.client.Respawn(ctx, &internal.JasperProcessID{Value: p.info.Id})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &rpcProcess{client: p.client, info: newProc}, nil
}

func (p *rpcProcess) RegisterTrigger(ctx context.Context, _ jasper.ProcessTrigger) error {
	return errors.New("cannot register triggers on remote processes")
}

func (p *rpcProcess) RegisterSignalTrigger(ctx context.Context, _ jasper.SignalTrigger) error {
	return errors.New("cannot register signal triggers on remote processes")
}

func (p *rpcProcess) RegisterSignalTriggerID(ctx context.Context, sigID jasper.SignalTriggerID) error {
	resp, err := p.client.RegisterSignalTriggerID(ctx, &internal.SignalTriggerParams{
		ProcessID:       &internal.JasperProcessID{Value: p.info.Id},
		SignalTriggerID: internal.ConvertSignalTriggerID(sigID),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (p *rpcProcess) Tag(tag string) {
	_, _ = p.client.TagProcess(context.Background(), &internal.ProcessTags{
		ProcessID: p.info.Id,
		Tags:      []string{tag},
	})
}

func (p *rpcProcess) GetTags() []string {
	tags, err := p.client.GetTags(context.Background(), &internal.JasperProcessID{Value: p.info.Id})
	if err != nil {
		return nil
	}

	return tags.Tags
}

func (p *rpcProcess) ResetTags() {
	_, _ = p.client.ResetTags(context.Background(), &internal.JasperProcessID{Value: p.info.Id})
}

type rpcScripting struct {
	id     string
	client internal.JasperProcessManagerClient
}

func (s *rpcScripting) ID() string { return s.id }

func (s *rpcScripting) Run(ctx context.Context, args []string) error {
	resp, err := s.client.ScriptingHarnessRun(ctx, &internal.ScriptingHarnessRunArgs{Id: s.id, Args: args})
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (s *rpcScripting) Setup(ctx context.Context) error {
	resp, err := s.client.ScriptingHarnessSetup(ctx, &internal.ScriptingHarnessID{Id: s.id})
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (s *rpcScripting) RunScript(ctx context.Context, script string) error {
	resp, err := s.client.ScriptingHarnessRunScript(ctx, &internal.ScriptingHarnessRunScriptArgs{Id: s.id, Script: script})
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}

func (s *rpcScripting) Build(ctx context.Context, dir string, args []string) (string, error) {
	resp, err := s.client.ScriptingHarnessBuild(ctx, &internal.ScriptingHarnessBuildArgs{Id: s.id, Directory: dir, Args: args})
	if err != nil {
		return "", errors.WithStack(err)
	}

	if !resp.Outcome.Success {
		return "", errors.New(resp.Outcome.Text)
	}

	return resp.Path, nil
}

func (s *rpcScripting) Test(ctx context.Context, dir string, args ...scripting.TestOptions) ([]scripting.TestResult, error) {
	resp, err := s.client.ScriptingHarnessTest(ctx, &internal.ScriptingHarnessTestArgs{Id: s.id, Directory: dir, Options: internal.ConvertScriptingTestOptions(args)})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if !resp.Outcome.Success {
		return nil, errors.New(resp.Outcome.Text)
	}

	return resp.Export()
}

func (s *rpcScripting) Cleanup(ctx context.Context) error {
	resp, err := s.client.ScriptingHarnessCleanup(ctx, &internal.ScriptingHarnessID{Id: s.id})
	if err != nil {
		return errors.WithStack(err)
	}

	if !resp.Success {
		return errors.New(resp.Text)
	}

	return nil
}
