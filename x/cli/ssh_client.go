package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
	"github.com/tychoish/jasper/x/remote"
	roptions "github.com/tychoish/jasper/x/remote/options"
	"github.com/tychoish/jasper/x/track"
)

// sshClient uses SSH to access a remote machine's Jasper CLI, which has access
// to methods in the RemoteClient interface.
type sshClient struct {
	manager jasper.Manager
	opts    sshClientOptions
	shCache scripting.HarnessCache
}

// NewSSHClient creates a new Jasper manager that connects to a remote
// machine's Jasper service over SSH using the remote machine's Jasper CLI.
func NewSSHClient(remoteOpts options.Remote, clientOpts ClientOptions, trackProcs bool) (remote.Manager, error) {
	if err := remoteOpts.Validate(); err != nil {
		return nil, fmt.Errorf("problem validating remote options: %w", err)
	}
	for _, arg := range remoteOpts.Args {
		if strings.HasPrefix(arg, "-v") {
			return nil, errors.New("cannot use verbose arguments in non-interactive SSH client")
		}
	}
	// We have to run SSH without output, because it will prevent the JSON
	// output from the Jasper CLI from being parsed correctly (e.g. adding a
	// host to the known hosts file generates a warning).
	remoteOpts.Args = append(remoteOpts.Args,
		"-T",
		"-o", "LogLevel=QUIET",
	)

	if err := clientOpts.Validate(); err != nil {
		return nil, fmt.Errorf("problem validating client options: %w", err)
	}

	id := uuid.New().String()
	var tracker jasper.ProcessTracker
	var err error
	if trackProcs {
		tracker, err = track.New(id)
		if err != nil {
			return nil, err
		}
	}

	manager := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{ID: id, Tracker: tracker}))

	client := &sshClient{
		opts: sshClientOptions{
			Machine: remoteOpts,
			Client:  clientOpts,
		},
		shCache: scripting.NewCache(),
		manager: manager,
	}
	return client, nil
}

func (c *sshClient) ID() string {
	output, err := c.runManagerCommand(context.Background(), IDCommand, nil)
	if err != nil {
		return ""
	}

	resp, err := ExtractIDResponse(output)
	if err != nil {
		return ""
	}

	return resp.ID
}

func (c *sshClient) CreateProcess(ctx context.Context, opts *options.Create) (jasper.Process, error) {
	output, err := c.runManagerCommand(ctx, CreateProcessCommand, opts)
	if err != nil {
		return nil, err
	}

	resp, err := ExtractInfoResponse(output)
	if err != nil {
		return nil, err
	}

	return newSSHProcess(c.runClientCommand, resp.Info)
}

// CreateCommand creates a command that logically will execute via the remote
// CLI. Users should not use (*jasper.Command).SetRunFunc().
func (c *sshClient) CreateCommand(ctx context.Context) *jasper.Command {
	return c.manager.CreateCommand(ctx).SetRunFunc(func(opts options.Command) error {
		output, err := c.runManagerCommand(ctx, CreateCommand, &opts)
		if err != nil {
			return fmt.Errorf("could not run command from given input: %w", err)
		}

		if _, err := ExtractOutcomeResponse(output); err != nil {
			return err
		}

		return nil
	})
}

// TODO (EVG-12616): fix this.
func (c *sshClient) CreateScripting(ctx context.Context, opts options.ScriptingHarness) (scripting.Harness, error) {
	return c.shCache.Create(c.manager, opts)
}

// TODO (EVG-12616): fix this.
func (c *sshClient) GetScripting(ctx context.Context, id string) (scripting.Harness, error) {
	return c.shCache.Get(id)
}

func (c *sshClient) Register(ctx context.Context, proc jasper.Process) error {
	return errors.New("cannot register existing processes on remote manager")
}

func (c *sshClient) List(ctx context.Context, f options.Filter) ([]jasper.Process, error) {
	output, err := c.runManagerCommand(ctx, ListCommand, &FilterInput{Filter: f})
	if err != nil {
		return nil, err
	}

	resp, err := ExtractInfosResponse(output)
	if err != nil {
		return nil, err
	}

	procs := make([]jasper.Process, len(resp.Infos))
	for i := range resp.Infos {
		if procs[i], err = newSSHProcess(c.runClientCommand, resp.Infos[i]); err != nil {
			return nil, fmt.Errorf("problem creating SSH process: %w", err)
		}
	}

	return procs, nil
}

func (c *sshClient) Group(ctx context.Context, tag string) ([]jasper.Process, error) {
	output, err := c.runManagerCommand(ctx, GroupCommand, &TagInput{Tag: tag})
	if err != nil {
		return nil, err
	}

	resp, err := ExtractInfosResponse(output)
	if err != nil {
		return nil, err
	}

	procs := make([]jasper.Process, len(resp.Infos))
	for i := range resp.Infos {
		if procs[i], err = newSSHProcess(c.runClientCommand, resp.Infos[i]); err != nil {
			return nil, fmt.Errorf("problem creating SSH process: %w", err)
		}
	}

	return procs, nil
}

func (c *sshClient) Get(ctx context.Context, id string) (jasper.Process, error) {
	output, err := c.runManagerCommand(ctx, GetCommand, &IDInput{ID: id})
	if err != nil {
		return nil, err
	}

	resp, err := ExtractInfoResponse(output)
	if err != nil {
		return nil, err
	}

	return newSSHProcess(c.runClientCommand, resp.Info)
}

func (c *sshClient) Clear(ctx context.Context) {
	_, _ = c.runManagerCommand(ctx, ClearCommand, nil)
}

func (c *sshClient) Close(ctx context.Context) error {
	output, err := c.runManagerCommand(ctx, CloseCommand, nil)
	if err != nil {
		return err
	}

	if _, err = ExtractOutcomeResponse(output); err != nil {
		return err
	}

	return nil
}

func (c *sshClient) CloseConnection() error {
	return nil
}

func (c *sshClient) DownloadFile(ctx context.Context, opts roptions.Download) error {
	output, err := c.runRemoteCommand(ctx, DownloadFileCommand, &opts)
	if err != nil {
		return err
	}

	if _, err := ExtractOutcomeResponse(output); err != nil {
		return err
	}

	return nil
}

func (c *sshClient) WriteFile(ctx context.Context, opts options.WriteFile) error {
	return opts.WriteBufferedContent(func(opts options.WriteFile) error {
		output, err := c.runRemoteCommand(ctx, WriteFileCommand, &opts)
		if err != nil {
			return err
		}

		if _, err := ExtractOutcomeResponse(output); err != nil {
			return err
		}

		return nil
	})
}

func (c *sshClient) GetLogStream(ctx context.Context, id string, count int) (jasper.LogStream, error) {
	output, err := c.runRemoteCommand(ctx, GetLogStreamCommand, &LogStreamInput{ID: id, Count: count})
	if err != nil {
		return jasper.LogStream{}, err
	}

	resp, err := ExtractLogStreamResponse(output)
	if err != nil {
		return resp.LogStream, err
	}

	return resp.LogStream, nil
}

func (c *sshClient) SignalEvent(ctx context.Context, name string) error {
	output, err := c.runRemoteCommand(ctx, SignalEventCommand, &EventInput{Name: name})
	if err != nil {
		return err
	}

	if _, err := ExtractOutcomeResponse(output); err != nil {
		return err
	}

	return nil
}

// TODO (EVG-12626): fix this.
func (c *sshClient) LoggingCache(ctx context.Context) jasper.LoggingCache {
	return c.manager.LoggingCache(ctx)
}

func (c *sshClient) SendMessages(ctx context.Context, opts options.LoggingPayload) error {
	output, err := c.runRemoteCommand(ctx, SendMessagesCommand, opts)
	if err != nil {
		return err
	}

	if _, err := ExtractOutcomeResponse(output); err != nil {
		return err
	}

	return nil
}

func (c *sshClient) runManagerCommand(ctx context.Context, managerSubcommand string, subcommandInput interface{}) ([]byte, error) {
	return c.runClientCommand(ctx, []string{ManagerCommand, managerSubcommand}, subcommandInput)
}

func (c *sshClient) runRemoteCommand(ctx context.Context, remoteSubcommand string, subcommandInput interface{}) ([]byte, error) {
	return c.runClientCommand(ctx, []string{RemoteCommand, remoteSubcommand}, subcommandInput)
}

// runClientCommand creates a command that runs the given CLI client subcommand
// over SSH with the given input to be sent as JSON to standard input. If
// subcommandInput is nil, it does not use standard input.
func (c *sshClient) runClientCommand(ctx context.Context, subcommand []string, subcommandInput interface{}) ([]byte, error) {
	input, err := clientInput(subcommandInput)
	if err != nil {
		return nil, fmt.Errorf("problem creating client input: %w", err)
	}
	output := clientOutput()

	cmd := c.newCommand(ctx, subcommand, input, output)
	if err := cmd.Run(ctx); err != nil {
		return nil, fmt.Errorf("problem running command '%s' over SSH: %w", c.opts.buildCommand(subcommand...), err)
	}

	return output.Bytes(), nil
}

// newCommand creates the command that runs the Jasper CLI client command
// over SSH.
func (c *sshClient) newCommand(ctx context.Context, clientSubcommand []string, input []byte, output io.WriteCloser) *jasper.Command {
	cmd := c.manager.CreateCommand(ctx).SetRemoteOptions(&c.opts.Machine).
		Add(c.opts.buildCommand(clientSubcommand...))

	if len(input) != 0 {
		cmd.SetInputBytes(input)
	}

	if output != nil {
		cmd.SetCombinedWriter(output)
	}

	return cmd
}

// clientOutput constructs the buffer to write the CLI output.
func clientOutput() *CappedWriter {
	return &CappedWriter{
		Buffer:   &bytes.Buffer{},
		MaxBytes: 1024 * 1024, // 1 MB
	}
}

// clientInput constructs the JSON input to the CLI from the struct.
func clientInput(input interface{}) ([]byte, error) {
	if input == nil {
		return nil, nil
	}

	inputBytes, err := json.MarshalIndent(input, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("could not encode input as JSON: %w", err)
	}

	return inputBytes, nil
}
