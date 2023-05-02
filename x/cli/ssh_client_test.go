package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/mock"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
	"github.com/tychoish/jasper/testutil"
	roptions "github.com/tychoish/jasper/x/remote/options"
)

func TestNewSSHClient(t *testing.T) {
	for testName, testCase := range map[string]func(t *testing.T, remoteOpts options.Remote, clientOpts ClientOptions){
		"NewSSHClientFailsWithEmptyRemoteOptions": func(t *testing.T, _ options.Remote, clientOpts ClientOptions) {
			remoteOpts := options.Remote{}
			_, err := NewSSHClient(remoteOpts, clientOpts, false)
			assert.Error(t, err)
		},
		"NewSSHClientFailsWithEmptyClientOptions": func(t *testing.T, remoteOpts options.Remote, _ ClientOptions) {
			clientOpts := ClientOptions{}
			_, err := NewSSHClient(remoteOpts, clientOpts, false)
			assert.Error(t, err)
		},
		"NewSSHClientSucceedsWithPopulatedOptions": func(t *testing.T, remoteOpts options.Remote, clientOpts ClientOptions) {
			client, err := NewSSHClient(remoteOpts, clientOpts, false)
			assert.NotError(t, err)
			assert.True(t, client != nil)
		},
	} {
		t.Run(testName, func(t *testing.T) {
			testCase(t, mockRemoteOptions(), mockClientOptions())
		})
	}
}

func TestSSHClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager){
		"VerifyBaseFixtureFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			opts := options.Create{
				Args: []string{"foo", "bar"},
			}
			proc, err := client.CreateProcess(ctx, &opts)
			assert.Error(t, err)
			assert.True(t, proc == nil)
		},
		"CreateProcessPassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			opts := options.Create{
				Args: []string{"foo", "bar"},
			}
			info := jasper.ProcessInfo{
				ID:        "the_created_process",
				IsRunning: true,
				Options:   opts,
			}

			inputChecker := options.Create{}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, CreateProcessCommand},
				&inputChecker,
				&InfoResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Info:            info,
				},
			)

			proc, err := client.CreateProcess(ctx, &opts)
			assert.NotError(t, err)

			assert.EqualItems(t, opts.Args, inputChecker.Args)

			sshProc, ok := proc.(*sshProcess)
			assert.True(t, ok)

			assert.Equal(t, info.ID, sshProc.info.ID)
		},
		"CreateProcessFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			opts := options.Create{Args: []string{"foo", "bar"}}
			_, err := client.CreateProcess(ctx, &opts)
			assert.Error(t, err)
		},
		"CreateProcessFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, CreateProcessCommand},
				nil,
				invalidResponse(),
			)
			opts := options.Create{
				Args: []string{"foo", "bar"},
			}
			_, err := client.CreateProcess(ctx, &opts)
			assert.Error(t, err)
		},
		"RunCommandPassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			inputChecker := options.Command{}

			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, CreateCommand},
				&inputChecker,
				makeOutcomeResponse(nil),
			)
			cmd := []string{"echo", "foo"}
			assert.NotError(t, client.CreateCommand(ctx).Add(cmd).Run(ctx))

			assert.Equal(t, len(inputChecker.Commands), 1)
			assert.EqualItems(t, cmd, inputChecker.Commands[0])
		},
		"RunCommandFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			assert.Error(t, client.CreateCommand(ctx).Add([]string{"echo", "foo"}).Run(ctx))
		},
		"RunCommandFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, CreateCommand},
				nil,
				invalidResponse(),
			)
			assert.Error(t, client.CreateCommand(ctx).Add([]string{"echo", "foo"}).Run(ctx))
		},
		"RegisterFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			assert.Error(t, client.Register(ctx, &mock.Process{}))
		},
		"ListPassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			runningInfo := jasper.ProcessInfo{
				ID:        "running",
				IsRunning: true,
			}
			successfulInfo := jasper.ProcessInfo{
				ID:         "successful",
				Complete:   true,
				Successful: true,
			}

			inputChecker := FilterInput{}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, ListCommand},
				&inputChecker,
				&InfosResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Infos:           []jasper.ProcessInfo{runningInfo, successfulInfo},
				},
			)
			filter := options.All
			procs, err := client.List(ctx, filter)
			assert.NotError(t, err)
			assert.Equal(t, filter, inputChecker.Filter)

			runningFound := false
			successfulFound := false
			for _, proc := range procs {
				sshProc, ok := proc.(*sshProcess)
				assert.True(t, ok)
				if sshProc.info.ID == runningInfo.ID {
					runningFound = true
				}
				if sshProc.info.ID == successfulInfo.ID {
					successfulFound = true
				}
			}
			check.True(t, runningFound)
			check.True(t, successfulFound)
		},
		"ListFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			_, err := client.List(ctx, options.All)
			assert.Error(t, err)
		},
		"ListFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, ListCommand},
				nil,
				invalidResponse(),
			)
			_, err := client.List(ctx, options.All)
			assert.Error(t, err)
		},
		"GroupPassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			info := jasper.ProcessInfo{
				ID:        "running",
				IsRunning: true,
			}

			inputChecker := TagInput{}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, GroupCommand},
				&inputChecker,
				&InfosResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Infos:           []jasper.ProcessInfo{info},
				},
			)
			tag := "foo"
			procs, err := client.Group(ctx, tag)
			assert.NotError(t, err)
			assert.Equal(t, tag, inputChecker.Tag)

			assert.Equal(t, len(procs), 1)
			sshProc, ok := procs[0].(*sshProcess)
			assert.True(t, ok)
			assert.Equal(t, info.ID, sshProc.info.ID)
		},
		"GroupFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			_, err := client.Group(ctx, "foo")
			assert.Error(t, err)
		},
		"GroupFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, GroupCommand},
				nil,
				invalidResponse(),
			)
			_, err := client.Group(ctx, "foo")
			assert.Error(t, err)
		},
		"GetPassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			id := "foo"
			info := jasper.ProcessInfo{
				ID: id,
			}
			inputChecker := IDInput{}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, GetCommand},
				&inputChecker,
				&InfoResponse{
					OutcomeResponse: *makeOutcomeResponse(nil),
					Info:            info,
				},
			)
			proc, err := client.Get(ctx, id)
			assert.NotError(t, err)
			assert.Equal(t, id, inputChecker.ID)

			sshProc, ok := proc.(*sshProcess)
			assert.True(t, ok)
			assert.Equal(t, info.ID, sshProc.info.ID)
		},
		"GetFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			_, err := client.Get(ctx, "foo")
			assert.Error(t, err)
		},
		"GetFailsWIthInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, GetCommand},
				nil,
				invalidResponse(),
			)
			_, err := client.Get(ctx, "foo")
			assert.Error(t, err)
		},
		"ClearPasses": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, ClearCommand},
				nil,
				&struct{}{},
			)
			client.Clear(ctx)
		},
		"ClosePassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, CloseCommand},
				nil,
				makeOutcomeResponse(nil),
			)
			assert.NotError(t, client.Close(ctx))
		},
		"CloseFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			assert.Error(t, client.Close(ctx))
		},
		"CloseFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{ManagerCommand, CloseCommand},
				nil,
				invalidResponse(),
			)
			assert.Error(t, client.Close(ctx))
		},
		"CloseConnectionPasses": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			check.NotError(t, client.CloseConnection())
		},
		"DownloadFilePassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			inputChecker := roptions.Download{}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, DownloadFileCommand},
				&inputChecker,
				makeOutcomeResponse(nil),
			)
			opts := roptions.Download{URL: "https://example.com", Path: "/foo"}
			assert.NotError(t, client.DownloadFile(ctx, opts))

			assert.Equal(t, opts, inputChecker)
		},
		"DownloadFileFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, DownloadFileCommand},
				nil,
				invalidResponse(),
			)
			opts := roptions.Download{URL: "https://example.com", Path: "/foo"}
			assert.Error(t, client.DownloadFile(ctx, opts))
		},
		"DownloadFileFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			opts := roptions.Download{URL: "https://example.com", Path: "/foo"}
			assert.Error(t, client.DownloadFile(ctx, opts))
		},
		"GetLogStreamPassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			inputChecker := LogStreamInput{}
			resp := &LogStreamResponse{
				LogStream:       jasper.LogStream{Logs: []string{"foo"}, Done: true},
				OutcomeResponse: *makeOutcomeResponse(nil),
			}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, GetLogStreamCommand},
				&inputChecker,
				resp,
			)
			id := "foo"
			count := 10
			logs, err := client.GetLogStream(ctx, id, count)
			assert.NotError(t, err)

			assert.Equal(t, id, inputChecker.ID)
			assert.Equal(t, count, inputChecker.Count)

			assert.EqualItems(t, logs.Logs, resp.LogStream.Logs)
		},
		"GetLogStreamFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, GetLogStreamCommand},
				nil,
				invalidResponse(),
			)
			_, err := client.GetLogStream(ctx, "foo", 10)
			assert.Error(t, err)
		},
		"GetLogStreamFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			_, err := client.GetLogStream(ctx, "foo", 10)
			assert.Error(t, err)
		},
		"SignalEventPassesWithValidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			inputChecker := EventInput{}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, SignalEventCommand},
				&inputChecker,
				makeOutcomeResponse(nil),
			)
			name := "foo"
			assert.NotError(t, client.SignalEvent(ctx, name))
			assert.Equal(t, name, inputChecker.Name)
		},
		"SignalEventFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, SignalEventCommand},
				nil,
				invalidResponse(),
			)
			assert.Error(t, client.SignalEvent(ctx, "foo"))
		},
		"SignalEventFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			assert.Error(t, client.SignalEvent(ctx, "foo"))
		},
		"WriteFileSucceeds": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			inputChecker := options.WriteFile{}
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, WriteFileCommand},
				&inputChecker,
				makeOutcomeResponse(nil),
			)

			opts := options.WriteFile{Path: filepath.Join(testutil.BuildDirectory(), "write_file"), Content: []byte("foo")}
			assert.NotError(t, client.WriteFile(ctx, opts))
		},
		"WriteFileFailsWithInvalidResponse": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.Create = makeCreateFunc(
				t, client,
				[]string{RemoteCommand, WriteFileCommand},
				nil,
				invalidResponse(),
			)
			opts := options.WriteFile{Path: filepath.Join(testutil.BuildDirectory(), "write_file"), Content: []byte("foo")}
			assert.Error(t, client.WriteFile(ctx, opts))
		},
		"WriteFileFailsIfBaseManagerCreateFails": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			baseManager.FailCreate = true
			opts := options.WriteFile{Path: filepath.Join(testutil.BuildDirectory(), "write_file"), Content: []byte("foo")}
			assert.Error(t, client.WriteFile(ctx, opts))
		},
		"ScriptingReturnsFromCache": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {
			opts := &options.ScriptingPython{
				VirtualEnvPath: testutil.BuildDirectory(),
				Packages:       []string{"pymongo"},
			}
			env, err := scripting.NewHarness(baseManager, opts)
			assert.NotError(t, err)
			assert.NotError(t, client.shCache.Add(env.ID(), env))

			sh, err := client.CreateScripting(ctx, opts)
			assert.NotError(t, err)
			assert.True(t, sh != nil)
			assert.Equal(t, env.ID(), sh.ID())
		},
		// "": func(ctx context.Context, t *testing.T, client *sshClient, baseManager *mock.Manager) {},
	} {
		t.Run(testName, func(t *testing.T) {
			client, err := NewSSHClient(mockRemoteOptions(), mockClientOptions(), false)
			assert.NotError(t, err)
			sshClient, ok := client.(*sshClient)
			assert.True(t, ok)

			mockManager := &mock.Manager{}
			sshClient.manager = jasper.Manager(mockManager)

			tctx, cancel := context.WithTimeout(ctx, testutil.TestTimeout)
			defer cancel()

			testCase(tctx, t, sshClient, mockManager)
		})
	}
}

// makeCreateFunc creates the function for the mock manager that reads the
// standard input (if the command accepts JSON input from standard input and
// inputChecker is non-nil) and verifies that it can be unmarshaled into the
// inputChecker for verification by the caller, verifies that the
// expectedClientSubcommand is the CLI command that is being run, and writes the
// expectedResponse back to the user.
func makeCreateFunc(t *testing.T, client *sshClient, expectedClientSubcommand []string, inputChecker interface{}, expectedResponse interface{}) func(*options.Create) mock.Process {
	return func(opts *options.Create) mock.Process {
		if opts.StandardInputBytes != nil && inputChecker != nil {
			input, err := io.ReadAll(bytes.NewBuffer(opts.StandardInputBytes))
			assert.NotError(t, err)
			assert.NotError(t, json.Unmarshal(input, inputChecker))
		}

		cliCommand := strings.Join(client.opts.buildCommand(expectedClientSubcommand...), " ")
		assert.Equal(t, cliCommand, strings.Join(opts.Args, " "))
		assert.True(t, expectedResponse != nil)
		assert.NotError(t, writeOutput(opts.Output.Output, expectedResponse))
		return mock.Process{}
	}
}

func invalidResponse() interface{} {
	return &struct{}{}
}

func mockRemoteOptions() options.Remote {
	opts := options.Remote{}
	opts.User = "user"
	opts.Host = "localhost"
	opts.Port = 12345
	opts.Password = "abc123"
	return opts
}

func mockClientOptions() ClientOptions {
	return ClientOptions{
		BinaryPath: "binary",
		Type:       RPCService,
	}
}
