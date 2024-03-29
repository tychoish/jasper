package mock

import (
	"context"
	"fmt"
	"runtime"

	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/mock"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
	roptions "github.com/tychoish/jasper/x/remote/options"
)

// RemoteClient implements the RemoteClient interface with exported fields
// to configure and introspect the mock's behavior.
type RemoteClient struct {
	mock.Manager
	FailCloseConnection bool
	FailDownloadFile    bool
	FailGetLogStream    bool
	FailSignalEvent     bool
	FailCreateScripting bool
	FailGetScripting    bool
	FailSendMessages    bool

	// DownloadFile input
	DownloadOptions roptions.Download

	// LogStream input/output
	LogStreamID    string
	LogStreamCount int
	jasper.LogStream

	EventName string

	SendMessagePayload options.LoggingPayload
}

// CloseConnection is a no-op. If FailCloseConnection is set, it returns an
// error.
func (c *RemoteClient) CloseConnection() error {
	if c.FailCloseConnection {
		return mockFail()
	}
	return nil
}

// DownloadFile stores the given download options. If FailDownloadFile is set,
// it returns an error.
func (c *RemoteClient) DownloadFile(ctx context.Context, opts roptions.Download) error {
	if c.FailDownloadFile {
		return mockFail()
	}

	c.DownloadOptions = opts

	return nil
}

// GetLogStream stores the given log stream ID and count and returns a
// jasper.LogStream indicating that it is done. If FailGetLogStream is set, it
// returns an error.
func (c *RemoteClient) GetLogStream(ctx context.Context, id string, count int) (jasper.LogStream, error) {
	if c.FailGetLogStream {
		return jasper.LogStream{Done: true}, mockFail()
	}

	c.LogStreamID = id
	c.LogStreamCount = count

	return c.LogStream, nil
}

// SignalEvent stores the given event name. If FailSignalEvent is set, it
// returns an error.
func (c *RemoteClient) SignalEvent(ctx context.Context, name string) error {
	if c.FailSignalEvent {
		return mockFail()
	}

	c.EventName = name

	return nil
}

// SendMessages stores the given logging payload. If FailSendMessages is set, it
// returns an error.
func (c *RemoteClient) SendMessages(ctx context.Context, opts options.LoggingPayload) error {
	if c.FailSendMessages {
		return mockFail()
	}

	c.SendMessagePayload = opts
	return nil
}

// GetScripting returns a cached scripting environment. If FailGetScripting is
// set, it returns an error.
func (c *RemoteClient) GetScripting(ctx context.Context, id string) (scripting.Harness, error) {
	if c.FailGetScripting {
		return nil, mockFail()
	}
	return c.ScriptingEnv, nil
}

// CreateScripting constructs an attached scripting environment. If
// FailCreateScripting is set, it returns an error.
func (c *RemoteClient) CreateScripting(ctx context.Context, opts options.ScriptingHarness) (scripting.Harness, error) {
	if c.FailCreateScripting {
		return nil, mockFail()
	}
	return c.ScriptingEnv, nil
}

func mockFail() error {
	progCounter := make([]uintptr, 2)
	n := runtime.Callers(2, progCounter)
	frames := runtime.CallersFrames(progCounter[:n])
	frame, _ := frames.Next()
	return fmt.Errorf("function failed: %s", frame.Function)
}
