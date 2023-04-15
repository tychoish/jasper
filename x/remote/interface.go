package remote

import (
	"context"

	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
)

// Manager provides an interface to access all functionality from a Jasper
// service. It includes an interface to interact with Jasper Managers and
// Processes remotely as well as access to remote-specific functionality.
type Manager interface {
	jasper.Manager

	CloseConnection() error
	DownloadFile(ctx context.Context, opts options.Download) error
	GetLogStream(ctx context.Context, id string, count int) (jasper.LogStream, error)
	SignalEvent(ctx context.Context, name string) error

	CreateScripting(context.Context, options.ScriptingHarness) (scripting.Harness, error)
	GetScripting(context.Context, string) (scripting.Harness, error)

	SendMessages(context.Context, options.LoggingPayload) error
}
