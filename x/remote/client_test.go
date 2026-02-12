package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mholt/archiver"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
	"github.com/tychoish/jasper/testutil"
	ropts "github.com/tychoish/jasper/x/remote/options"
)

func init() {
	sender := grip.Sender()
	sender.SetPriority(level.Info)
	grip.SetSender(sender)
}

type clientTestCase struct {
	Name string
	Case func(context.Context, *testing.T, Manager)
}

func addBasicClientTests(modify testutil.OptsModify, tests ...clientTestCase) []clientTestCase {
	return append([]clientTestCase{
		{
			Name: "ValidateFixture",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				check.True(t, ctx != nil)
				check.True(t, client != nil)
			},
		},
		{
			Name: "IDReturnsNonempty",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				check.NotZero(t, client.ID())
			},
		},
		{
			Name: "ProcEnvVarMatchesManagerID",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)
				info := proc.Info(ctx)
				assert.NotEqual(t, info.Options.Environment.Len(), 0)
				mapping := stw.Map[string, string]{}
				mapping.Extend(irt.KVsplit(info.Options.Environment.IteratorFront()))
				check.Equal(t, client.ID(), mapping[jasper.ManagerEnvironID])
			},
		},
		{
			Name: "CreateProcessFailsWithEmptyOptions",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := &options.Create{}
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.Error(t, err)
				check.True(t, proc == nil)
			},
		},
		{
			Name: "LongRunningOperationsAreListedAsRunning",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.SleepCreateOpts(20)
				modify(opts)
				procs, err := createProcs(ctx, opts, client, 10)
				assert.NotError(t, err)
				check.Equal(t, len(procs), 10)

				procs, err = client.List(ctx, options.All)
				assert.NotError(t, err)
				check.Equal(t, len(procs), 10)

				procs, err = client.List(ctx, options.Running)
				assert.NotError(t, err)
				check.Equal(t, len(procs), 10)

				procs, err = client.List(ctx, options.Successful)
				assert.NotError(t, err)
				check.Equal(t, len(procs), 0)
			},
		},
		{
			Name: "ListDoesNotErrorWhenEmptyResult",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				all, err := client.List(ctx, options.All)
				assert.NotError(t, err)
				check.Equal(t, len(all), 0)
			},
		},
		{
			Name: "ListErrorsWithInvalidFilter",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				procs, err := client.List(ctx, options.Filter("foo"))
				check.Error(t, err)
				check.True(t, procs == nil)
			},
		},
		{
			Name: "ListAllReturnsErrorWithCanceledContext",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				cctx, cancel := context.WithCancel(ctx)
				opts := testutil.TrueCreateOpts()
				modify(opts)
				created, err := createProcs(ctx, opts, client, 10)
				assert.NotError(t, err)
				check.Equal(t, len(created), 10)
				cancel()
				output, err := client.List(cctx, options.All)
				assert.Error(t, err)
				check.True(t, output == nil)
			},
		},
		{
			Name: "ListReturnsOneSuccessfulCommand",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)

				_, err = proc.Wait(ctx)
				assert.NotError(t, err)

				listOut, err := client.List(ctx, options.Successful)
				assert.NotError(t, err)

				assert.Equal(t, len(listOut), 1)
				check.Equal(t, listOut[0].ID(), proc.ID())
			},
		},
		{
			Name: "RegisterAlwaysErrors",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				proc, err := client.CreateProcess(ctx, &options.Create{Args: []string{"ls"}})
				check.True(t, proc != nil)
				assert.NotError(t, err)

				check.Error(t, client.Register(ctx, nil))
				check.Error(t, client.Register(ctx, proc))
			},
		},
		{
			Name: "GetMethodErrorsWithNoResponse",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				proc, err := client.Get(ctx, "foo")
				assert.Error(t, err)
				check.True(t, proc == nil)
			},
		},
		{
			Name: "GetMethodReturnsMatchingProc",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)

				ret, err := client.Get(ctx, proc.ID())
				assert.NotError(t, err)
				check.Equal(t, ret.ID(), proc.ID())
			},
		},
		{
			Name: "GroupDoesNotErrorWhenEmptyResult",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				procs, err := client.Group(ctx, "foo")
				assert.NotError(t, err)
				check.Equal(t, len(procs), 0)
			},
		},
		{
			Name: "GroupErrorsForCanceledContexts",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)
				_, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)

				cctx, cancel := context.WithCancel(ctx)
				cancel()
				procs, err := client.Group(cctx, "foo")
				assert.Error(t, err)
				check.Equal(t, len(procs), 0)
				check.Substring(t, err.Error(), "canceled")
			},
		},
		{
			Name: "GroupPropagatesMatching",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)

				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)

				proc.Tag("foo")

				procs, err := client.Group(ctx, "foo")
				assert.NotError(t, err)
				assert.Equal(t, len(procs), 1)
				check.Equal(t, procs[0].ID(), proc.ID())
			},
		},
		{
			Name: "CloseEmptyManagerNoops",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				assert.NotError(t, client.Close(ctx))
			},
		},
		{
			Name: "ClosersWithoutTriggersTerminatesProcesses",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				if runtime.GOOS == "windows" {
					t.Skip("the sleep tests don't block correctly on windows")
				}
				opts := testutil.SleepCreateOpts(100)
				modify(opts)

				_, err := createProcs(ctx, opts, client, 10)
				assert.NotError(t, err)
				check.NotError(t, client.Close(ctx))
			},
		},
		{
			Name: "CloseErrorsWithCanceledContext",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.SleepCreateOpts(100)
				modify(opts)

				_, err := createProcs(ctx, opts, client, 10)
				assert.NotError(t, err)

				cctx, cancel := context.WithCancel(ctx)
				cancel()

				err = client.Close(cctx)
				assert.Error(t, err)
				check.Substring(t, err.Error(), "canceled")
			},
		},
		{
			Name: "CloseSucceedsWithTerminatedProcesses",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				procs, err := createProcs(ctx, testutil.TrueCreateOpts(), client, 10)
				for _, p := range procs {
					_, err = p.Wait(ctx)
					assert.NotError(t, err)
				}

				assert.NotError(t, err)
				check.NotError(t, client.Close(ctx))
			},
		},
		{
			Name: "WaitingOnNonExistentProcessErrors",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)

				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)

				_, err = proc.Wait(ctx)
				assert.NotError(t, err)

				client.Clear(ctx)

				_, err = proc.Wait(ctx)
				assert.Error(t, err)
				procs, err := client.List(ctx, options.All)
				assert.NotError(t, err)
				check.Equal(t, len(procs), 0)
			},
		},
		{
			Name: "ClearCausesDeletionOfProcesses",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)
				sameProc, err := client.Get(ctx, proc.ID())
				assert.NotError(t, err)
				assert.Equal(t, proc.ID(), sameProc.ID())
				_, err = proc.Wait(ctx)
				assert.NotError(t, err)
				client.Clear(ctx)
				nilProc, err := client.Get(ctx, proc.ID())
				assert.Error(t, err)
				check.True(t, nilProc == nil)
			},
		},
		{
			Name: "ClearIsANoopForActiveProcesses",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.SleepCreateOpts(20)
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)
				client.Clear(ctx)
				sameProc, err := client.Get(ctx, proc.ID())
				assert.NotError(t, err)
				check.Equal(t, proc.ID(), sameProc.ID())
				assert.NotError(t, jasper.Terminate(ctx, proc)) // Clean up
			},
		},
		{
			Name: "ClearSelectivelyDeletesOnlyDeadProcesses",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				trueOpts := testutil.TrueCreateOpts()
				modify(trueOpts)
				lsProc, err := client.CreateProcess(ctx, trueOpts)
				assert.NotError(t, err)

				sleepOpts := testutil.SleepCreateOpts(20)
				modify(sleepOpts)
				sleepProc, err := client.CreateProcess(ctx, sleepOpts)
				assert.NotError(t, err)

				_, err = lsProc.Wait(ctx)
				assert.NotError(t, err)

				client.Clear(ctx)

				sameSleepProc, err := client.Get(ctx, sleepProc.ID())
				assert.NotError(t, err)
				check.Equal(t, sleepProc.ID(), sameSleepProc.ID())

				nilProc, err := client.Get(ctx, lsProc.ID())
				assert.Error(t, err)
				check.True(t, nilProc == nil)
				assert.NotError(t, jasper.Terminate(ctx, sleepProc)) // Clean up
			},
		},
		{
			Name: "RegisterIsDisabled",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				err := client.Register(ctx, nil)
				assert.Error(t, err)
				check.Substring(t, err.Error(), "cannot register")
			},
		},
		{
			Name: "CreateProcessReturnsCorrectExample",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.TrueCreateOpts()
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)
				check.True(t, proc != nil)
				check.NotZero(t, proc.ID())

				fetched, err := client.Get(ctx, proc.ID())
				assert.NotError(t, err)
				assert.True(t, fetched != nil)
				check.Equal(t, proc.ID(), fetched.ID())
			},
		},
		{
			Name: "WaitOnSigKilledProcessReturnsProperExitCode",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := testutil.SleepCreateOpts(100)
				modify(opts)
				proc, err := client.CreateProcess(ctx, opts)
				assert.NotError(t, err)
				assert.True(t, proc != nil)
				assert.NotZero(t, proc.ID())

				assert.NotError(t, proc.Signal(ctx, syscall.SIGKILL))

				exitCode, err := proc.Wait(ctx)
				assert.Error(t, err)
				if runtime.GOOS == "windows" {
					check.Equal(t, 1, exitCode)
				} else {
					check.Equal(t, 9, exitCode)
				}
			},
		},
		{
			Name: "WriteFileFailsWithInvalidPath",
			Case: func(ctx context.Context, t *testing.T, client Manager) {
				opts := options.WriteFile{Content: []byte("foo")}
				check.Error(t, client.WriteFile(ctx, opts))
			},
		},
	}, tests...)
}

func TestManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, factory := range []struct {
		Name        string
		Constructor func(context.Context, *testing.T) Manager
	}{
		{
			Name: "MDB",
			Constructor: func(ctx context.Context, t *testing.T) Manager {
				t.SkipNow()
				mngr := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

				client, err := makeTestMDBServiceAndClient(ctx, mngr)
				assert.NotError(t, err)
				return client
			},
		},
		{
			Name: "RPC/TLS",
			Constructor: func(ctx context.Context, t *testing.T) Manager {
				mngr := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

				client, err := makeTLSRPCServiceAndClient(ctx, mngr)
				assert.NotError(t, err)
				return client
			},
		},
		{
			Name: "RPC/Insecure",
			Constructor: func(ctx context.Context, t *testing.T) Manager {
				check.NotPanic(t, func() {
					newRPCClient(nil)
				})

				mngr := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))

				client, err := makeInsecureRPCServiceAndClient(ctx, mngr)
				assert.NotError(t, err)
				return client
			},
		},
	} {
		t.Run(factory.Name, func(t *testing.T) {
			for _, modify := range []struct {
				Name    string
				Options testutil.OptsModify
			}{
				{
					Name: "Blocking",
					Options: func(opts *options.Create) {
						opts.Implementation = options.ProcessImplementationBlocking
					},
				},
				{
					Name: "Basic",
					Options: func(opts *options.Create) {
						opts.Implementation = options.ProcessImplementationBasic
					},
				},
				{
					Name:    "Default",
					Options: func(opts *options.Create) {},
				},
			} {
				t.Run(modify.Name, func(t *testing.T) {
					for _, test := range addBasicClientTests(modify.Options,
						clientTestCase{
							Name: "StandardInput",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								for subTestName, subTestCase := range map[string]func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte){
									"ReaderIsIgnored": func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte) {
										opts.StandardInput = bytes.NewBuffer(stdin)

										proc, err := client.CreateProcess(ctx, opts)
										assert.NotError(t, err)

										_, err = proc.Wait(ctx)
										assert.NotError(t, err)

										logs, err := client.GetLogStream(ctx, proc.ID(), 1)
										assert.NotError(t, err)
										check.Equal(t, 0, len(logs.Logs))
									},
									"BytesSetsStandardInput": func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte) {
										opts.StandardInputBytes = stdin

										proc, err := client.CreateProcess(ctx, opts)
										assert.NotError(t, err)

										_, err = proc.Wait(ctx)
										assert.NotError(t, err)

										logs, err := client.GetLogStream(ctx, proc.ID(), 1)
										assert.NotError(t, err)

										assert.Equal(t, len(logs.Logs), 1)
										check.Equal(t, expectedOutput, strings.TrimSpace(logs.Logs[0]))
									},
									"BytesCopiedByRespawnedProcess": func(ctx context.Context, t *testing.T, opts *options.Create, expectedOutput string, stdin []byte) {
										opts.StandardInputBytes = stdin

										proc, err := client.CreateProcess(ctx, opts)
										assert.NotError(t, err)

										_, err = proc.Wait(ctx)
										assert.NotError(t, err)

										logs, err := client.GetLogStream(ctx, proc.ID(), 1)
										assert.NotError(t, err)

										assert.Equal(t, len(logs.Logs), 1)
										check.Equal(t, expectedOutput, strings.TrimSpace(logs.Logs[0]))

										newProc, err := proc.Respawn(ctx)
										assert.NotError(t, err)

										_, err = newProc.Wait(ctx)
										assert.NotError(t, err)

										logs, err = client.GetLogStream(ctx, newProc.ID(), 1)
										assert.NotError(t, err)

										assert.Equal(t, len(logs.Logs), 1)
										check.Equal(t, expectedOutput, strings.TrimSpace(logs.Logs[0]))
									},
								} {
									t.Run(subTestName, func(t *testing.T) {
										inMemLogger, err := jasper.NewInMemoryLogger(1)
										assert.NotError(t, err)

										opts := &options.Create{
											Args: []string{"bash", "-s"},
											Output: options.Output{
												Loggers: []*options.LoggerConfig{inMemLogger},
											},
										}
										modify.Options(opts)

										expectedOutput := "foobar"
										stdin := []byte("echo " + expectedOutput)
										subTestCase(ctx, t, opts, expectedOutput, stdin)
									})
								}
							},
						},
						clientTestCase{
							Name: "WriteFileSucceeds",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpFile, err := os.CreateTemp(testutil.BuildDirectory(), filepath.Base(t.Name()))
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpFile.Name()))
								}()
								assert.NotError(t, tmpFile.Close())

								opts := options.WriteFile{Path: tmpFile.Name(), Content: []byte("foo")}
								assert.NotError(t, client.WriteFile(ctx, opts))

								content, err := os.ReadFile(tmpFile.Name())
								assert.NotError(t, err)

								check.Equal(t, string(opts.Content), string(content))
							},
						},
						clientTestCase{
							Name: "WriteFileAcceptsContentFromReader",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpFile, err := os.CreateTemp(testutil.BuildDirectory(), filepath.Base(t.Name()))
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpFile.Name()))
								}()
								assert.NotError(t, tmpFile.Close())

								buf := []byte("foo")
								opts := options.WriteFile{Path: tmpFile.Name(), Reader: bytes.NewBuffer(buf)}
								assert.NotError(t, client.WriteFile(ctx, opts))

								content, err := os.ReadFile(tmpFile.Name())
								assert.NotError(t, err)

								check.Equal(t, string(buf), string(content))
							},
						},
						clientTestCase{
							Name: "WriteFileSucceedsWithLargeContent",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpFile, err := os.CreateTemp(testutil.BuildDirectory(), filepath.Base(t.Name()))
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpFile.Name()))
								}()
								assert.NotError(t, tmpFile.Close())

								const mb = 1024 * 1024
								opts := options.WriteFile{Path: tmpFile.Name(), Content: bytes.Repeat([]byte("foo"), mb)}
								assert.NotError(t, client.WriteFile(ctx, opts))

								content, err := os.ReadFile(tmpFile.Name())
								assert.NotError(t, err)

								check.Equal(t, string(opts.Content), string(content))
							},
						},
						clientTestCase{
							Name: "WriteFileSucceedsWithLargeContentFromReader",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpFile, err := os.CreateTemp(testutil.BuildDirectory(), filepath.Base(t.Name()))
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, tmpFile.Close())
									check.NotError(t, os.RemoveAll(tmpFile.Name()))
								}()

								const mb = 1024 * 1024
								buf := bytes.Repeat([]byte("foo"), 2*mb)
								opts := options.WriteFile{Path: tmpFile.Name(), Reader: bytes.NewBuffer(buf)}
								assert.NotError(t, client.WriteFile(ctx, opts))

								content, err := os.ReadFile(tmpFile.Name())
								assert.NotError(t, err)

								check.Equal(t, string(buf), string(content))
							},
						},
						clientTestCase{
							Name: "WriteFileSucceedsWithNoContent",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								path := filepath.Join(testutil.BuildDirectory(), filepath.Base(t.Name()))
								assert.NotError(t, os.RemoveAll(path))
								defer func() {
									check.NotError(t, os.RemoveAll(path))
								}()

								opts := options.WriteFile{Path: path}
								assert.NotError(t, client.WriteFile(ctx, opts))

								stat, err := os.Stat(path)
								assert.NotError(t, err)

								check.Zero(t, stat.Size())
							},
						},
						clientTestCase{
							Name: "GetLogStreamFromNonexistentProcessFails",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								stream, err := client.GetLogStream(ctx, "foo", 1)
								check.Error(t, err)
								check.True(t, !stream.Done && stream.Logs == nil)
							},
						},
						clientTestCase{
							Name: "GetLogStreamFailsWithoutInMemoryLogger",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								opts := &options.Create{Args: []string{"echo", "foo"}}
								modify.Options(opts)
								proc, err := client.CreateProcess(ctx, opts)
								assert.NotError(t, err)
								assert.True(t, proc != nil)

								_, err = proc.Wait(ctx)
								assert.NotError(t, err)

								_, err = client.GetLogStream(ctx, proc.ID(), 1)
								check.Error(t, err)
							},
						},
						clientTestCase{
							Name: "WithInMemoryLogger",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								inMemLogger, err := jasper.NewInMemoryLogger(100)
								assert.NotError(t, err)
								output := "foo"
								opts := &options.Create{
									Args: []string{"echo", output},
									Output: options.Output{
										Loggers: []*options.LoggerConfig{inMemLogger},
									},
								}
								modify.Options(opts)

								for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, proc jasper.Process){
									"GetLogStreamFailsForInvalidCount": func(ctx context.Context, t *testing.T, proc jasper.Process) {
										_, err := client.GetLogStream(ctx, proc.ID(), -1)
										check.Error(t, err)
									},
									"GetLogStreamReturnsOutputOnSuccess": func(ctx context.Context, t *testing.T, proc jasper.Process) {
										logs := []string{}
										for stream, err := client.GetLogStream(ctx, proc.ID(), 1); !stream.Done; stream, err = client.GetLogStream(ctx, proc.ID(), 1) {
											assert.NotError(t, err)
											assert.NotEqual(t, len(stream.Logs), 0)
											logs = append(logs, stream.Logs...)
										}
										check.Contains(t, logs, output)
									},
								} {
									t.Run(testName, func(t *testing.T) {
										proc, err := client.CreateProcess(ctx, opts)
										assert.NotError(t, err)
										assert.True(t, proc != nil)

										_, err = proc.Wait(ctx)
										assert.NotError(t, err)
										testCase(ctx, t, proc)
									})
								}
							},
						},
						clientTestCase{
							Name: "DownloadFile",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, client Manager, tempDir string){
									"CreatesFileIfNonexistent": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										opts := ropts.Download{
											URL:  "https://example.com",
											Path: filepath.Join(tempDir, filepath.Base(t.Name())),
										}
										assert.NotError(t, client.DownloadFile(ctx, opts))
										defer func() {
											check.NotError(t, os.RemoveAll(opts.Path))
										}()

										fileInfo, err := os.Stat(opts.Path)
										assert.NotError(t, err)
										check.NotZero(t, fileInfo.Size())
									},
									"WritesFileIfExists": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										file, err := os.CreateTemp(tempDir, "out.txt")
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(file.Name()))
										}()
										assert.NotError(t, file.Close())

										opts := ropts.Download{
											URL:  "https://example.com",
											Path: file.Name(),
										}
										assert.NotError(t, client.DownloadFile(ctx, opts))
										defer func() {
											check.NotError(t, os.RemoveAll(opts.Path))
										}()

										fileInfo, err := os.Stat(file.Name())
										assert.NotError(t, err)
										check.NotZero(t, fileInfo.Size())
									},
									"CreatesFileAndExtracts": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										downloadDir, err := os.MkdirTemp(tempDir, "out")
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(downloadDir))
										}()

										fileServerDir, err := os.MkdirTemp(tempDir, "file_server")
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(fileServerDir))
										}()

										fileName := "foo.zip"
										fileContents := "foo"
										assert.NotError(t, AddFileToDirectory(fileServerDir, fileName, fileContents))

										absDownloadDir, err := filepath.Abs(downloadDir)
										assert.NotError(t, err)
										destFilePath := filepath.Join(absDownloadDir, fileName)
										destExtractDir := filepath.Join(absDownloadDir, "extracted")

										port := testutil.GetPortNumber()
										fileServerAddr := fmt.Sprintf("localhost:%d", port)
										fileServer := &http.Server{Addr: fileServerAddr, Handler: http.FileServer(http.Dir(fileServerDir))}
										defer func() {
											check.NotError(t, fileServer.Close())
										}()
										listener, err := net.Listen("tcp", fileServerAddr)
										assert.NotError(t, err)
										go func() {
											grip.Info(fileServer.Serve(listener))
										}()

										baseURL := fmt.Sprintf("http://%s", fileServerAddr)
										assert.NotError(t, testutil.WaitForRESTService(ctx, baseURL))

										opts := ropts.Download{
											URL:  fmt.Sprintf("%s/%s", baseURL, fileName),
											Path: destFilePath,
											ArchiveOpts: ropts.Archive{
												ShouldExtract: true,
												Format:        ropts.ArchiveZip,
												TargetPath:    destExtractDir,
											},
										}
										assert.NotError(t, client.DownloadFile(ctx, opts))

										fileInfo, err := os.Stat(destFilePath)
										assert.NotError(t, err)
										check.NotZero(t, fileInfo.Size())

										dirContents, err := os.ReadDir(destExtractDir)
										assert.NotError(t, err)

										check.NotZero(t, len(dirContents))
									},
									"FailsForInvalidArchiveFormat": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										file, err := os.CreateTemp(tempDir, filepath.Base(t.Name()))
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(file.Name()))
										}()
										assert.NotError(t, file.Close())
										extractDir, err := os.MkdirTemp(tempDir, filepath.Base(t.Name())+"_extract")
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(file.Name()))
										}()

										opts := ropts.Download{
											URL:  "https://example.com",
											Path: file.Name(),
											ArchiveOpts: ropts.Archive{
												ShouldExtract: true,
												Format:        ropts.ArchiveFormat("foo"),
												TargetPath:    extractDir,
											},
										}
										check.Error(t, client.DownloadFile(ctx, opts))
									},
									"FailsForUnarchivedFile": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										extractDir, err := os.MkdirTemp(tempDir, filepath.Base(t.Name())+"_extract")
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(extractDir))
										}()
										opts := ropts.Download{
											URL:  "https://example.com",
											Path: filepath.Join(tempDir, filepath.Base(t.Name())),
											ArchiveOpts: ropts.Archive{
												ShouldExtract: true,
												Format:        ropts.ArchiveAuto,
												TargetPath:    extractDir,
											},
										}
										check.Error(t, client.DownloadFile(ctx, opts))

										dirContents, err := os.ReadDir(extractDir)
										assert.NotError(t, err)
										check.Zero(t, len(dirContents))
									},
									"FailsForInvalidURL": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										file, err := os.CreateTemp(tempDir, filepath.Base(t.Name()))
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(file.Name()))
										}()
										assert.NotError(t, file.Close())
										check.Error(t, client.DownloadFile(ctx, ropts.Download{URL: "", Path: file.Name()}))
									},
									"FailsForNonexistentURL": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										file, err := os.CreateTemp(tempDir, "out.txt")
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(file.Name()))
										}()
										assert.NotError(t, file.Close())
										check.Error(t, client.DownloadFile(ctx, ropts.Download{URL: "https://example.com/foo", Path: file.Name()}))
									},
									"FailsForInsufficientPermissions": func(ctx context.Context, t *testing.T, client Manager, tempDir string) {
										if os.Geteuid() == 0 {
											t.Skip("cannot test download permissions as root")
										} else if runtime.GOOS == "windows" {
											t.Skip("cannot test download permissions on windows")
										}
										check.Error(t, client.DownloadFile(ctx, ropts.Download{URL: "https://example.com", Path: "/foo/bar"}))
									},
								} {
									t.Run(testName, func(t *testing.T) {
										tempDir, err := os.MkdirTemp(testutil.BuildDirectory(), filepath.Base(t.Name()))
										assert.NotError(t, err)
										defer func() {
											check.NotError(t, os.RemoveAll(tempDir))
										}()
										testCase(ctx, t, client, tempDir)
									})
								}
							},
						},
						clientTestCase{
							Name: "CreateWithLogFile",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								file, err := os.CreateTemp(testutil.BuildDirectory(), filepath.Base(t.Name()))
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(file.Name()))
								}()
								assert.NotError(t, file.Close())

								logger := &options.LoggerConfig{}
								assert.NotError(t, logger.Set(&options.FileLoggerOptions{
									Filename: file.Name(),
									Base:     options.BaseOptions{Format: options.LogFormatPlain},
								}))
								output := "foobar"
								opts := &options.Create{
									Args: []string{"echo", output},
									Output: options.Output{
										Loggers: []*options.LoggerConfig{logger},
									},
								}

								proc, err := client.CreateProcess(ctx, opts)
								assert.NotError(t, err)

								exitCode, err := proc.Wait(ctx)
								assert.NotError(t, err)
								assert.Zero(t, exitCode)

								info, err := os.Stat(file.Name())
								assert.NotError(t, err)
								check.NotZero(t, info.Size())

								fileContents, err := os.ReadFile(file.Name())
								assert.NotError(t, err)
								check.Substring(t, string(fileContents), output)
							},
						},
						clientTestCase{
							Name: "RegisterSignalTriggerIDChecksForInvalidTriggerID",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								proc, err := client.CreateProcess(ctx, testutil.SleepCreateOpts(1))
								assert.NotError(t, err)
								check.True(t, proc.Running(ctx))

								check.Error(t, proc.RegisterSignalTriggerID(ctx, jasper.SignalTriggerID("foo")))

								check.NotError(t, proc.Signal(ctx, syscall.SIGTERM))
							},
						},
						clientTestCase{
							Name: "RegisterSignalTriggerIDPassesWithValidArgs",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								proc, err := client.CreateProcess(ctx, testutil.SleepCreateOpts(1))
								assert.NotError(t, err)
								check.True(t, proc.Running(ctx))

								check.NotError(t, proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger))

								check.NotError(t, proc.Signal(ctx, syscall.SIGTERM))
							},
						},
						clientTestCase{
							Name: "LoggingCacheCreate",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								logger, err := lc.Create("new_logger", &options.Output{})
								assert.NotError(t, err)
								check.Equal(t, "new_logger", logger.ID)

								// should fail with existing logger
								_, err = lc.Create("new_logger", &options.Output{})
								check.Error(t, err)
							},
						},
						clientTestCase{
							Name: "LoggingCachePutNotImplemented",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								check.Error(t, lc.Put("logger", &options.CachedLogger{ID: "logger"}))
							},
						},
						clientTestCase{
							Name: "LoggingCacheGetExists",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								expectedLogger, err := lc.Create("new_logger", &options.Output{})
								assert.NotError(t, err)

								logger := lc.Get(expectedLogger.ID)
								assert.True(t, logger != nil)
								check.Equal(t, expectedLogger.ID, logger.ID)
							},
						},
						clientTestCase{
							Name: "LoggingCacheGetDNE",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								logger := lc.Get("DNE")
								assert.True(t, logger == nil)
							},
						},
						clientTestCase{
							Name: "LoggingCacheRemove",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								logger1, err := lc.Create("logger1", &options.Output{})
								assert.NotError(t, err)
								logger2, err := lc.Create("logger2", &options.Output{})
								assert.NotError(t, err)

								assert.True(t, lc.Get(logger1.ID) != nil)
								assert.True(t, lc.Get(logger2.ID) != nil)
								lc.Remove(logger2.ID)
								assert.True(t, lc.Get(logger1.ID) != nil)
								assert.True(t, lc.Get(logger2.ID) == nil)
							},
						},
						clientTestCase{
							Name: "LoggingCacheCloseAndRemoveSucceeds",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								logger1, err := lc.Create("logger1", &options.Output{})
								assert.NotError(t, err)
								logger2, err := lc.Create("logger2", &options.Output{})
								assert.NotError(t, err)

								assert.True(t, lc.Get(logger1.ID) != nil)
								assert.True(t, lc.Get(logger2.ID) != nil)
								assert.NotError(t, lc.CloseAndRemove(ctx, logger2.ID))
								assert.True(t, lc.Get(logger1.ID) != nil)
								assert.True(t, lc.Get(logger2.ID) == nil)
							},
						},
						clientTestCase{
							Name: "LoggingCacheClearSucceeds",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								logger1, err := lc.Create("logger1", &options.Output{})
								assert.NotError(t, err)
								logger2, err := lc.Create("logger2", &options.Output{})
								assert.NotError(t, err)

								assert.True(t, lc.Get(logger1.ID) != nil)
								assert.True(t, lc.Get(logger2.ID) != nil)
								assert.NotError(t, lc.Clear(ctx))
								assert.True(t, lc.Get(logger1.ID) == nil)
								assert.True(t, lc.Get(logger2.ID) == nil)
							},
						},
						clientTestCase{
							Name: "LoggingCachePruneSucceeds",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								logger1, err := lc.Create("logger1", &options.Output{})
								assert.NotError(t, err)
								assert.True(t, lc.Get(logger1.ID) != nil)
								time.Sleep(2 * time.Second)

								logger2, err := lc.Create("logger2", &options.Output{})
								assert.NotError(t, err)

								lc.Prune(time.Now().Add(-time.Second))
								assert.True(t, lc.Get(logger1.ID) == nil)
								assert.True(t, lc.Get(logger2.ID) != nil)
							},
						},
						clientTestCase{
							Name: "LoggingCacheLenEmpty",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								_, err := lc.Create("logger1", &options.Output{})
								assert.NotError(t, err)
								_, err = lc.Create("logger2", &options.Output{})
								assert.NotError(t, err)

								check.Equal(t, 2, lc.Len())
							},
						},
						clientTestCase{
							Name: "LoggingCacheLenNotEmpty",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								check.Zero(t, lc.Len())
							},
						},
						clientTestCase{
							Name: "LoggingSendMessagesInvalid",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								payload := options.LoggingPayload{
									LoggerID: "DNE",
									Data:     "new log message",
									Priority: level.Warning,
									Format:   options.LoggingPayloadFormatSTRING,
								}
								check.Error(t, client.SendMessages(ctx, payload))
							},
						},
						clientTestCase{
							Name: "LoggingSendMessagesValid",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								lc := client.LoggingCache(ctx)
								logger1, err := lc.Create("logger1", &options.Output{})
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, lc.Clear(ctx))
								}()

								payload := options.LoggingPayload{
									LoggerID: logger1.ID,
									Data:     "new log message",
									Priority: level.Warning,
									Format:   options.LoggingPayloadFormatSTRING,
								}
								check.NotError(t, client.SendMessages(ctx, payload))
							},
						},
						clientTestCase{
							Name: "ScriptingGetDNE",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								_, err := client.GetScripting(ctx, "DNE")
								check.Error(t, err)
							},
						},
						clientTestCase{
							Name: "ScriptingGetExists",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								expectedHarness := createTestScriptingHarness(ctx, t, client, ".")

								harness, err := client.GetScripting(ctx, expectedHarness.ID())
								assert.NotError(t, err)
								check.Equal(t, expectedHarness.ID(), harness.ID())
							},
						},
						clientTestCase{
							Name: "ScriptingSetup",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								harness := createTestScriptingHarness(ctx, t, client, ".")
								check.NotError(t, harness.Setup(ctx))
							},
						},
						clientTestCase{
							Name: "ScriptingCleanup",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								harness := createTestScriptingHarness(ctx, t, client, ".")
								check.NotError(t, harness.Cleanup(ctx))
							},
						},
						clientTestCase{
							Name: "ScriptingRunNoError",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpdir, err := os.MkdirTemp(testutil.BuildDirectory(), "scripting_tests")
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpdir))
								}()
								harness := createTestScriptingHarness(ctx, t, client, tmpdir)

								assert.NotError(t, err)
								tmpFile := filepath.Join(tmpdir, "fake_script.go")
								assert.NotError(t, os.WriteFile(tmpFile, []byte(`package main; import "os"; func main() { os.Exit(0) }`), 0o755))
								check.NotError(t, harness.Run(ctx, []string{tmpFile}))
							},
						},
						clientTestCase{
							Name: "ScriptingRunError",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpdir, err := os.MkdirTemp(testutil.BuildDirectory(), "scripting_tests")
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpdir))
								}()
								harness := createTestScriptingHarness(ctx, t, client, tmpdir)

								tmpFile := filepath.Join(tmpdir, "fake_script.go")
								assert.NotError(t, os.WriteFile(tmpFile, []byte(`package main; import "os"; func main() { os.Exit(42) }`), 0o755))
								check.Error(t, harness.Run(ctx, []string{tmpFile}))
							},
						},
						clientTestCase{
							Name: "ScriptingRunScriptNoError",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpdir, err := os.MkdirTemp(testutil.BuildDirectory(), "scripting_tests")
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpdir))
								}()

								harness := createTestScriptingHarness(ctx, t, client, tmpdir)
								check.NotError(t, harness.RunScript(ctx, `package main; import "fmt"; func main() { fmt.Println("Hello World") }`))
							},
						},
						clientTestCase{
							Name: "ScriptingRunScriptError",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpdir, err := os.MkdirTemp(testutil.BuildDirectory(), "scripting_tests")
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpdir))
								}()

								harness := createTestScriptingHarness(ctx, t, client, tmpdir)
								assert.Error(t, harness.RunScript(ctx, `package main; import "os"; func main() { os.Exit(42) }`))
							},
						},
						clientTestCase{
							Name: "ScriptingBuild",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpdir, err := os.MkdirTemp(testutil.BuildDirectory(), "scripting_tests")
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpdir))
								}()
								harness := createTestScriptingHarness(ctx, t, client, tmpdir)

								tmpFile := filepath.Join(tmpdir, "fake_script.go")
								assert.NotError(t, os.WriteFile(tmpFile, []byte(`package main; import "os"; func main() { os.Exit(0) }`), 0o755))
								_, err = harness.Build(ctx, tmpdir, []string{
									"-o",
									filepath.Join(tmpdir, "fake_script"),
									tmpFile,
								})
								assert.NotError(t, err)
								_, err = os.Stat(filepath.Join(tmpFile))
								assert.NotError(t, err)
							},
						},
						clientTestCase{
							Name: "ScriptingTest",
							Case: func(ctx context.Context, t *testing.T, client Manager) {
								tmpdir, err := os.MkdirTemp(testutil.BuildDirectory(), "scripting_tests")
								assert.NotError(t, err)
								defer func() {
									check.NotError(t, os.RemoveAll(tmpdir))
								}()
								harness := createTestScriptingHarness(ctx, t, client, tmpdir)

								tmpFile := filepath.Join(tmpdir, "fake_script_test.go")
								assert.NotError(t, os.WriteFile(tmpFile, []byte(`package main; import "testing"; func TestMain(t *testing.T) { return }`), 0o755))
								results, err := harness.Test(ctx, tmpdir, scripting.TestOptions{Name: "dummy"})
								assert.NotError(t, err)
								assert.Equal(t, len(results), 1)
							},
						},
					) {
						t.Run(test.Name, func(t *testing.T) {
							tctx, cancel := context.WithTimeout(ctx, testutil.RPCTestTimeout)
							defer cancel()
							test.Case(tctx, t, factory.Constructor(tctx, t))
						})
					}
				})
			}
		})
	}
}

func createTestScriptingHarness(ctx context.Context, t *testing.T, client Manager, dir string) scripting.Harness {
	opts := options.NewGolangScriptingEnvironment(filepath.Join(dir, "gopath"), runtime.GOROOT()).(*options.ScriptingGolang)

	opts.Output.Error = send.MakeWriterSender(grip.Sender())
	opts.Output.Output = send.MakeWriterSender(grip.Sender())

	harness, err := client.CreateScripting(ctx, opts)
	assert.NotError(t, err)

	return harness
}

// AddFileToDirectory adds an archive file given by fileName with the given
// fileContents to the directory.
func AddFileToDirectory(dir string, fileName string, fileContents string) error {
	if format, _ := archiver.ByExtension(fileName); format != nil {
		builder, ok := format.(archiver.Archiver)
		if !ok {
			return errors.New("unsupported archive format")
		}

		tmpFile, err := os.CreateTemp(dir, "tmp.txt")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpFile.Name())
		if _, err := tmpFile.Write([]byte(fileContents)); err != nil {
			catcher := &erc.Collector{}
			catcher.Push(err)
			catcher.Push(tmpFile.Close())
			return catcher.Resolve()
		}
		if err := tmpFile.Close(); err != nil {
			return err
		}

		if err := builder.Archive([]string{tmpFile.Name()}, filepath.Join(dir, fileName)); err != nil {
			return err
		}
		return nil
	}

	file, err := os.Create(filepath.Join(dir, fileName))
	if err != nil {
		return err
	}
	if _, err := file.Write([]byte(fileContents)); err != nil {
		catcher := &erc.Collector{}
		catcher.Push(err)
		catcher.Push(file.Close())
		return catcher.Resolve()
	}
	return file.Close()
}
