package cli

import (
	"context"
	"encoding/json"
	"syscall"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
	"github.com/urfave/cli"
)

func TestCLIRemoteWindows(t *testing.T) {
	for remoteType, makeService := range map[string]func(ctx context.Context, t *testing.T, port int, manager jasper.Manager) util.CloseFunc{
		RESTService: makeTestRESTService,
		RPCService:  makeTestRPCService,
	} {
		t.Run(remoteType, func(t *testing.T) {
			for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *cli.Context){
				"SignalEventPasses": func(ctx context.Context, t *testing.T, c *cli.Context) {
					eventName := "event"
					utf16EventName, err := syscall.UTF16PtrFromString(eventName)
					assert.NotError(t, err)

					event, err := jasper.CreateEvent(utf16EventName)
					assert.NotError(t, err)
					defer jasper.CloseHandle(event)

					input, err := json.Marshal(EventInput{Name: eventName})
					assert.NotError(t, err)
					resp := &OutcomeResponse{}
					assert.NotError(t, execCLICommandInputOutput(t, c, remoteSignalEvent(), input, resp))
					check.True(t, resp.Successful())
				},
			} {

				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()

					port := testutil.GetPortNumber()
					c := mockCLIContext(remoteType, port)
					manager, err := jasper.NewSynchronizedManager(false)
					assert.NotError(t, err)
					closeService := makeService(ctx, t, port, manager)
					assert.NotError(t, err)
					defer func() {
						check.NotError(t, closeService())
					}()

					testCase(ctx, t, c)
				})
			}
		})
	}
}
