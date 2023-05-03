package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/mock"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
	roptions "github.com/tychoish/jasper/x/remote/options"
)

type neverJSON struct{}

func (n *neverJSON) MarshalJSON() ([]byte, error)  { return nil, errors.New("always error") }
func (n *neverJSON) UnmarshalJSON(in []byte) error { return errors.New("always error") }
func (n *neverJSON) Read(p []byte) (int, error)    { return 0, errors.New("always error") }
func (n *neverJSON) Close() error                  { return errors.New("always error") }

func TestRestService(t *testing.T) {
	httpClient := testutil.GetHTTPClient()
	defer testutil.PutHTTPClient(httpClient)

	tempDir, err := os.MkdirTemp(testutil.BuildDirectory(), filepath.Base(t.Name()))
	assert.NotError(t, err)
	defer func() { check.NotError(t, os.RemoveAll(tempDir)) }()

	for name, test := range map[string]func(context.Context, *testing.T, *Service, *restClient){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			assert.True(t, srv != nil)
			assert.True(t, client != nil)
			assert.True(t, srv.manager != nil)
			assert.True(t, client.client != nil)
			assert.NotZero(t, client.prefix)

			// similarly about helper functions
			client.prefix = ""
			assert.Equal(t, "/foo", client.getURL("foo"))
			_, err := makeBody(&neverJSON{})
			assert.Error(t, err)
			assert.Error(t, handleError(&http.Response{Body: &neverJSON{}, StatusCode: http.StatusTeapot}))
		},
		"ClientMethodsErrorWithBadURL": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			client.prefix = strings.Replace(client.prefix, "http://", "://", 1)

			_, err := client.List(ctx, options.All)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			_, err = client.CreateProcess(ctx, nil)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			_, err = client.Group(ctx, "foo")
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			_, err = client.Get(ctx, "foo")
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			err = client.Close(ctx)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			_, err = client.getProcessInfo(ctx, "foo")
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			_, err = client.GetLogStream(ctx, "foo", 1)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			err = client.DownloadFile(ctx, roptions.Download{URL: "foo", Path: "bar"})
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")
		},
		"ClientRequestsFailWithMalformedURL": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			client.prefix = strings.Replace(client.prefix, "http://", "http;//", 1)

			_, err := client.List(ctx, options.All)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			_, err = client.Group(ctx, "foo")
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			_, err = client.CreateProcess(ctx, nil)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			_, err = client.Get(ctx, "foo")
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			err = client.Close(ctx)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			_, err = client.getProcessInfo(ctx, "foo")
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			_, err = client.GetLogStream(ctx, "foo", 1)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			err = client.DownloadFile(ctx, roptions.Download{URL: "foo", Path: "bar"})
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")
		},
		"ProcessMethodsWithBadURL": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			client.prefix = strings.Replace(client.prefix, "http://", "://", 1)

			proc := &restProcess{
				client: client,
				id:     "foo",
			}

			err := proc.Signal(ctx, syscall.SIGTERM)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			_, err = proc.Wait(ctx)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")

			proc.Tag("a")

			out := proc.GetTags()
			assert.True(t, out == nil)

			proc.ResetTags()

			err = proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem building request")
		},
		"ProcessRequestsFailWithBadURL": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {

			client.prefix = strings.Replace(client.prefix, "http://", "http;//", 1)

			proc := &restProcess{
				client: client,
				id:     "foo",
			}

			err := proc.Signal(ctx, syscall.SIGTERM)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			_, err = proc.Wait(ctx)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")

			proc.Tag("a")

			out := proc.GetTags()
			assert.True(t, out == nil)

			proc.ResetTags()

			err = proc.RegisterSignalTriggerID(ctx, jasper.CleanTerminationSignalTrigger)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "problem making request")
		},
		"CheckSafetyOfTagMethodsForBrokenTasks": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			proc := &restProcess{
				client: client,
				id:     "foo",
			}

			proc.Tag("a")

			out := proc.GetTags()
			assert.True(t, out == nil)

			proc.ResetTags()
		},
		"SignalFailsForTaskThatDoesNotExist": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			proc := &restProcess{
				client: client,
				id:     "foo",
			}

			err := proc.Signal(ctx, syscall.SIGTERM)
			assert.Error(t, err)
			assert.Substring(t, err.Error(), "no process")
		},
		"CreateProcessEndpointErrorsWithMalformedData": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			body, err := makeBody(map[string]int{"tags": 42})
			assert.NotError(t, err)

			req, err := http.NewRequest(http.MethodPost, "", io.NopCloser(body))
			assert.NotError(t, err)
			rw := httptest.NewRecorder()
			srv.createProcess(rw, req)
			assert.Equal(t, http.StatusBadRequest, rw.Code)
		},
		"WaitForProcessThatDoesNotExistShouldError": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			proc := &restProcess{
				client: client,
				id:     "foo",
			}

			_, err := proc.Wait(ctx)
			assert.Error(t, err)
		},
		"SignalProcessThatDoesNotExistShouldError": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			proc := &restProcess{
				client: client,
				id:     "foo",
			}

			assert.Error(t, proc.Signal(ctx, syscall.SIGTERM))
		},
		"SignalErrorsWithInvalidSyscall": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			proc, err := client.CreateProcess(ctx, testutil.SleepCreateOpts(10))
			assert.NotError(t, err)

			assert.Error(t, proc.Signal(ctx, syscall.Signal(-1)))
		},
		"MetricsErrorForInvalidProcess": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			req, err := http.NewRequest(http.MethodGet, client.getURL("/process/%s/metrics", "foo"), nil)
			assert.NotError(t, err)
			req = req.WithContext(ctx)
			res, err := httpClient.Do(req)
			assert.NotError(t, err)

			assert.Equal(t, http.StatusNotFound, res.StatusCode)
		},
		"GetProcessWhenInvalid": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			_, err := client.Get(ctx, "foo")
			assert.Error(t, err)
		},
		"CreateFailPropagatesErrors": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			srv.manager = &mock.Manager{FailCreate: true}
			proc, err := client.CreateProcess(ctx, testutil.TrueCreateOpts())
			assert.Error(t, err)
			assert.True(t, proc == nil)
			assert.Substring(t, err.Error(), "problem submitting request")
		},
		"CreateFailsForTriggerReasons": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			srv.manager = &mock.Manager{
				CreateConfig: mock.Process{FailRegisterTrigger: true},
			}
			proc, err := client.CreateProcess(ctx, testutil.TrueCreateOpts())
			assert.Error(t, err)
			assert.True(t, proc == nil)
			assert.Substring(t, err.Error(), "problem registering trigger")
		},
		"MetricsPopulatedForValidProcess": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			id := "foo"
			srv.manager = &mock.Manager{
				Procs: []jasper.Process{
					&mock.Process{ProcInfo: jasper.ProcessInfo{ID: id, PID: os.Getpid()}},
				},
			}

			req, err := http.NewRequest(http.MethodGet, client.getURL("/process/%s/metrics", id), nil)
			assert.NotError(t, err)
			req = req.WithContext(ctx)
			res, err := httpClient.Do(req)
			assert.NotError(t, err)

			assert.Equal(t, http.StatusOK, res.StatusCode)
		},
		"AddTagsWithNoTagsSpecifiedShouldError": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			id := "foo"
			srv.manager = &mock.Manager{
				Procs: []jasper.Process{
					&mock.Process{ProcInfo: jasper.ProcessInfo{ID: id}},
				},
			}

			req, err := http.NewRequest(http.MethodPost, client.getURL("/process/%s/tags", id), nil)
			assert.NotError(t, err)
			req = req.WithContext(ctx)
			res, err := httpClient.Do(req)
			assert.NotError(t, err)

			assert.Equal(t, http.StatusBadRequest, res.StatusCode)

		},
		"SignalInPassingCase": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			id := "foo"
			srv.manager = &mock.Manager{
				Procs: []jasper.Process{
					&mock.Process{ProcInfo: jasper.ProcessInfo{ID: id}},
				},
			}
			proc := &restProcess{
				client: client,
				id:     id,
			}

			err := proc.Signal(ctx, syscall.SIGTERM)
			check.NotError(t, err)

		},
		"SignalFailsToParsePID": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			req, err := http.NewRequest(http.MethodPatch, client.getURL("/process/%s/signal/f", "foo"), nil)
			assert.NotError(t, err)
			req = req.WithContext(ctx)

			resp, err := client.client.Do(req)
			assert.NotError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Substring(t, handleError(resp).Error(), "problem converting signal 'f'")
		},
		"ServiceDownloadFileFailsWithInvalidOptions": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			body, err := makeBody(struct {
				URL int `json:"url"`
			}{URL: 0})
			assert.NotError(t, err)

			req, err := http.NewRequest(http.MethodPost, client.getURL("/download"), body)
			assert.NotError(t, err)
			rw := httptest.NewRecorder()
			srv.downloadFile(rw, req)
			assert.Equal(t, http.StatusBadRequest, rw.Code)
		},
		"GetLogStreamFailsWithoutInMemoryLogger": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			opts := &options.Create{Args: []string{"echo", "foo"}}

			proc, err := client.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			assert.True(t, proc != nil)

			_, err = proc.Wait(ctx)
			assert.NotError(t, err)

			_, err = client.GetLogStream(ctx, proc.ID(), 1)
			assert.Error(t, err)
		},
		"CreateWithMultipleLoggers": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			file, err := os.CreateTemp(tempDir, "out.txt")
			assert.NotError(t, err)
			defer func() {
				check.NotError(t, file.Close())
				check.NotError(t, os.RemoveAll(file.Name()))
			}()

			fileLogger := &options.LoggerConfig{}
			assert.NotError(t, fileLogger.Set(&options.FileLoggerOptions{
				Filename: file.Name(),
				Base:     options.BaseOptions{Format: options.LogFormatPlain},
			}))

			inMemoryLogger := &options.LoggerConfig{}
			assert.NotError(t, inMemoryLogger.Set(&options.InMemoryLoggerOptions{
				InMemoryCap: 100,
				Base:        options.BaseOptions{Format: options.LogFormatPlain},
			}))

			opts := &options.Create{Output: options.Output{Loggers: []*options.LoggerConfig{inMemoryLogger, fileLogger}}}
			opts.Args = []string{"echo", "foobar"}
			proc, err := client.CreateProcess(ctx, opts)
			assert.NotError(t, err)
			_, err = proc.Wait(ctx)
			assert.NotError(t, err)

			stream, err := client.GetLogStream(ctx, proc.ID(), 1)
			assert.NotError(t, err)
			assert.Equal(t, len(stream.Logs), 0)
			assert.True(t, !stream.Done)

			info, err := os.Stat(file.Name())
			assert.NotError(t, err)
			assert.NotZero(t, info.Size())

		},
		"WriteFileSucceeds": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			tmpFile, err := os.CreateTemp(tempDir, filepath.Base(t.Name()))
			assert.NotError(t, err)
			defer func() {
				check.NotError(t, tmpFile.Close())
				check.NotError(t, os.RemoveAll(tmpFile.Name()))
			}()

			opts := options.WriteFile{Path: tmpFile.Name(), Content: []byte("foo")}
			assert.NotError(t, client.WriteFile(ctx, opts))

			content, err := os.ReadFile(tmpFile.Name())
			assert.NotError(t, err)

			assert.Equal(t, string(opts.Content), string(content))
		},
		"WriteFileAcceptsContentFromReader": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			tmpFile, err := os.CreateTemp(tempDir, filepath.Base(t.Name()))
			assert.NotError(t, err)
			defer func() {
				check.NotError(t, tmpFile.Close())
				check.NotError(t, os.RemoveAll(tmpFile.Name()))
			}()

			buf := []byte("foo")
			opts := options.WriteFile{Path: tmpFile.Name(), Reader: bytes.NewBuffer(buf)}
			assert.NotError(t, client.WriteFile(ctx, opts))

			content, err := os.ReadFile(tmpFile.Name())
			assert.NotError(t, err)

			assert.Equal(t, string(buf), string(content))
		},
		"WriteFileSucceedsWithLargeContentReader": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			tmpFile, err := os.CreateTemp(tempDir, filepath.Base(t.Name()))
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

			assert.Equal(t, string(buf), string(content))
		},
		"RegisterSignalTriggerIDChecksForExistingProcess": func(ctx context.Context, t *testing.T, srv *Service, client *restClient) {
			req, err := http.NewRequest(http.MethodPatch, client.getURL("/process/%s/trigger/signal/%s", "foo", jasper.CleanTerminationSignalTrigger), nil)
			assert.NotError(t, err)

			resp, err := client.client.Do(req)
			assert.NotError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Substring(t, handleError(resp).Error(), "no process 'foo' found")
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), testutil.LongTestTimeout)
			defer cancel()

			srv, port, err := startRESTService(ctx, httpClient)
			assert.NotError(t, err)
			assert.True(t, srv != nil)

			assert.NotError(t, err)
			client := &restClient{
				prefix: fmt.Sprintf("http://localhost:%d/jasper/v1", port),
				client: httpClient,
			}

			test(ctx, t, srv, client)
		})
	}
}
