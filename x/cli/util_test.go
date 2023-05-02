package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/service"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/jasper/x/remote"
	"github.com/urfave/cli"
)

func TestReadInputValidJSON(t *testing.T) {
	input := bytes.NewBufferString(`{"foo":"bar","bat":"baz","qux":[1,2,3,4,5]}`)
	output := struct {
		Foo string `json:"foo"`
		Bat string `json:"bat"`
		Qux []int  `json:"qux"`
	}{}
	assert.NotError(t, readInput(input, &output))
	assert.Equal(t, "bar", output.Foo)
	assert.Equal(t, "baz", output.Bat)
	assert.EqualItems(t, []int{1, 2, 3, 4, 5}, output.Qux)
}

func TestReadInputInvalidInput(t *testing.T) {
	input := bytes.NewBufferString(`{"foo":}`)
	output := struct {
		Foo string `json:"foo"`
	}{}
	assert.Error(t, readInput(input, &output))
}

func TestReadInputInvalidOutput(t *testing.T) {
	input := bytes.NewBufferString(`{"foo":"bar"}`)
	output := make(chan struct{})
	assert.Error(t, readInput(input, output))
}

func TestWriteOutput(t *testing.T) {
	input := struct {
		Foo string `json:"foo"`
		Bat string `json:"bat"`
		Qux []int  `json:"qux"`
	}{
		Foo: "bar",
		Bat: "baz",
		Qux: []int{1, 2, 3, 4, 5},
	}
	inputBuf := bytes.NewBufferString(`
	{
	"foo": "bar",
	"bat": "baz",
	"qux": [1 ,2, 3, 4, 5]
	}
	`)
	inputString := inputBuf.String()
	output := &bytes.Buffer{}
	assert.NotError(t, writeOutput(output, input))
	assert.Equal(t, testutil.RemoveWhitespace(inputString), testutil.RemoveWhitespace(output.String()))
}

func TestWriteOutputInvalidInput(t *testing.T) {
	input := make(chan struct{})
	output := &bytes.Buffer{}
	assert.Error(t, writeOutput(output, input))
}

func TestWriteOutputInvalidOutput(t *testing.T) {
	input := bytes.NewBufferString(`{"foo":"bar"}`)

	output, err := os.CreateTemp(testutil.BuildDirectory(), "write_output.txt")
	assert.NotError(t, err)
	defer os.RemoveAll(output.Name())
	assert.NotError(t, output.Close())
	assert.Error(t, writeOutput(output, input))
}

func TestMakeRemoteClientInvalidService(t *testing.T) {
	ctx := context.Background()
	client, err := newRemoteClient(ctx, "invalid", "localhost", testutil.GetPortNumber(), "")
	assert.Error(t, err)
	assert.True(t, client == nil)
}

func TestMakeRemoteClient(t *testing.T) {
	for remoteType, makeServiceAndClient := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) (util.CloseFunc, remote.Manager){
		RESTService: makeTestRESTServiceAndClient,
		RPCService:  makeTestRPCServiceAndClient,
	} {
		t.Run(remoteType, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
			defer cancel()
			manager, err := jasper.NewSynchronizedManager(false)
			assert.NotError(t, err)
			closeService, client := makeServiceAndClient(ctx, t, testutil.GetPortNumber(), manager)
			check.NotError(t, closeService())
			check.NotError(t, client.CloseConnection())
		})
	}
}

func TestCLICommon(t *testing.T) {
	for remoteType, makeServiceAndClient := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) (util.CloseFunc, remote.Manager){
		RESTService: makeTestRESTServiceAndClient,
		RPCService:  makeTestRPCServiceAndClient,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) error{
				"CreateProcessWithConnection": func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) error {
					return withConnection(ctx, c, func(client remote.Manager) error {
						proc, err := client.CreateProcess(ctx, testutil.TrueCreateOpts())
						assert.NotError(t, err)
						assert.True(t, proc != nil)
						assert.NotZero(t, proc.Info(ctx).PID)
						return nil
					})
				},
				"DoPassthroughInputOutputReadsFromStdin": func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) error {
					return withMockStdin(t, `{"value":"foo"}`, func(stdin *os.File) error {
						return withMockStdout(t, func(*os.File) error {
							input := &mockInput{}
							assert.NotError(t, doPassthroughInputOutput(c, input, mockRequest("")))
							output, err := io.ReadAll(stdin)
							assert.NotError(t, err)
							assert.Equal(t, len(output), 0)
							return nil
						})
					})
				},
				"DoPassthroughInputOutputSetsAndValidatesInput": func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) error {
					expectedInput := "foo"
					return withMockStdin(t, fmt.Sprintf(`{"value":"%s"}`, expectedInput), func(*os.File) error {
						return withMockStdout(t, func(*os.File) error {
							input := &mockInput{}
							assert.NotError(t, doPassthroughInputOutput(c, input, mockRequest("")))
							assert.Equal(t, expectedInput, input.Value)
							check.True(t, input.validated)
							return nil
						})
					})
				},
				"DoPassthroughInputOutputWritesResponseToStdout": func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) error {
					return withMockStdin(t, `{"value":"foo"}`, func(*os.File) error {
						return withMockStdout(t, func(stdout *os.File) error {
							input := &mockInput{}
							outputVal := "bar"
							assert.NotError(t, doPassthroughInputOutput(c, input, mockRequest(outputVal)))
							assert.Equal(t, "foo", input.Value)
							check.True(t, input.validated)

							expectedOutput := `{"value":"bar"}`
							_, err := stdout.Seek(0, 0)
							assert.NotError(t, err)
							output, err := io.ReadAll(stdout)
							assert.NotError(t, err)
							assert.Equal(t, testutil.RemoveWhitespace(expectedOutput), testutil.RemoveWhitespace(string(output)))
							return nil
						})
					})
				},
				"DoPassthroughOutputIgnoresStdin": func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) error {
					input := "foo"
					return withMockStdin(t, input, func(stdin *os.File) error {
						return withMockStdout(t, func(*os.File) error {
							assert.NotError(t, doPassthroughOutput(c, mockRequest("")))
							output, err := io.ReadAll(stdin)
							assert.NotError(t, err)
							assert.Equal(t, len(output), len(input))
							return nil

						})
					})
				},
				"DoPassthroughOutputWritesResponseToStdout": func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) error {
					return withMockStdout(t, func(stdout *os.File) error {
						outputVal := "bar"
						assert.NotError(t, doPassthroughOutput(c, mockRequest(outputVal)))

						expectedOutput := `{"value": "bar"}`
						_, err := stdout.Seek(0, 0)
						assert.NotError(t, err)
						output, err := io.ReadAll(stdout)
						assert.NotError(t, err)
						assert.Equal(t, testutil.RemoveWhitespace(expectedOutput), testutil.RemoveWhitespace(string(output)))
						return nil
					})
				},
				// "": func(ctx context.Context, t *testing.T, c *cli.Context, client remote.Manager) err {},
			} {
				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()
					port := testutil.GetPortNumber()
					c := mockCLIContext(remoteType, port)
					manager, err := jasper.NewSynchronizedManager(false)
					assert.NotError(t, err)
					closeService, client := makeServiceAndClient(ctx, t, port, manager)
					defer func() {
						check.NotError(t, client.CloseConnection())
						check.NotError(t, closeService())
					}()

					check.NotError(t, testCase(ctx, t, c, client))
				})
			}
		})
	}
}

func TestWithService(t *testing.T) {
	svcFuncRan := false
	svcFunc := func(svc service.Service) error {
		svcFuncRan = true
		return nil
	}
	assert.Error(t, withService(&rpcDaemon{}, &service.Config{}, svcFunc))
	assert.True(t, !svcFuncRan)

	check.NotError(t, withService(&rpcDaemon{}, &service.Config{Name: "foo"}, svcFunc))
	check.True(t, svcFuncRan)
}

func TestRunServices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	check.NotError(t, runServices(ctx))
	assert.Equal(t, context.DeadlineExceeded, ctx.Err())

	ctx, cancel = context.WithCancel(context.Background())
	cancel()

	assert.Error(t, runServices(ctx, func(ctx context.Context) (util.CloseFunc, error) {
		return nil, ctx.Err()
	}))

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	check.NotError(t, runServices(ctx, func(ctx context.Context) (util.CloseFunc, error) {
		return func() error { return nil }, nil
	}))

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	closeFuncCalled := false
	closeFunc := func() error {
		closeFuncCalled = true
		return nil
	}

	assert.Error(t, runServices(ctx, func(ctx context.Context) (util.CloseFunc, error) {
		return closeFunc, errors.New("fail to make service")
	}))
	assert.True(t, !closeFuncCalled)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	closeFuncCalled = false

	check.NotError(t, runServices(ctx, func(ctx context.Context) (util.CloseFunc, error) {
		return closeFunc, nil
	}))
	check.True(t, closeFuncCalled)

	anotherCloseFuncCalled := false
	anotherCloseFunc := func() error {
		anotherCloseFuncCalled = true
		return nil
	}

	assert.Error(t, runServices(ctx, func(ctx context.Context) (util.CloseFunc, error) {
		return closeFunc, nil
	}, func(ctx context.Context) (util.CloseFunc, error) {
		return anotherCloseFunc, errors.New("fail to make another service")
	}))
	check.True(t, closeFuncCalled)
	assert.True(t, !anotherCloseFuncCalled)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	closeFuncCalled = false
	anotherCloseFuncCalled = false

	check.NotError(t, runServices(ctx, func(ctx context.Context) (util.CloseFunc, error) {
		return closeFunc, nil
	}, func(ctx context.Context) (util.CloseFunc, error) {
		return anotherCloseFunc, nil
	}))
	check.True(t, closeFuncCalled)
	check.True(t, anotherCloseFuncCalled)
}
