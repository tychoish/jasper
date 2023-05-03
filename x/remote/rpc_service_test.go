package remote

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/x/remote/internal"
	"google.golang.org/grpc"
)

func TestRPCService(t *testing.T) {
	for managerName, makeManager := range map[string]func() (jasper.Manager, error){
		"Basic": func() (jasper.Manager, error) {
			return jasper.NewManager(jasper.ManagerOptions{Synchronized: true}), nil
		},
	} {
		t.Run(managerName, func(t *testing.T) {
			for testName, testCase := range map[string]func(context.Context, *testing.T, internal.JasperProcessManagerClient){
				"RegisterSignalTriggerIDChecksForExistingProcess": func(ctx context.Context, t *testing.T, client internal.JasperProcessManagerClient) {
					outcome, err := client.RegisterSignalTriggerID(ctx, internal.ConvertSignalTriggerParams("foo", jasper.CleanTerminationSignalTrigger))
					assert.NotError(t, err)
					assert.True(t, !outcome.Success)
				},
				//"": func(ctx context.Context, t *testing.T, client internal.JasperProcessManagerClient) {},
			} {
				t.Run(testName, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
					defer cancel()

					manager, err := makeManager()
					assert.NotError(t, err)
					addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", testutil.GetPortNumber()))
					assert.NotError(t, err)
					assert.NotError(t, startTestRPCService(ctx, manager, addr, nil))

					conn, err := grpc.DialContext(ctx, addr.String(), grpc.WithInsecure(), grpc.WithBlock())
					assert.NotError(t, err)
					client := internal.NewJasperProcessManagerClient(conn)

					go func() {
						<-ctx.Done()
						check.NotError(t, conn.Close())
					}()

					testCase(ctx, t, client)
				})
			}
		})
	}
}
