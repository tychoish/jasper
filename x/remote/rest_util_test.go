package remote

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testutil"
)

func startRESTService(ctx context.Context, client *http.Client) (*Service, int, error) {
tryStartService:
	for {
		select {
		case <-ctx.Done():
			grip.Warning("timed out starting test service")
			return nil, -1, ctx.Err()
		default:
			synchronizedManager, err := jasper.NewSynchronizedManager(false)
			if err != nil {
				return nil, -1, err
			}
			srv := NewRestService(synchronizedManager)
			app := srv.App(ctx)
			app.SetPrefix("jasper")

			if err := app.SetHost("localhost"); err != nil {
				continue tryStartService
			}

			port := testutil.GetPortNumber()
			if err := app.SetPort(port); err != nil {
				continue tryStartService
			}

			go func() {
				grip.Warning(app.Run(ctx))
			}()

			timer := time.NewTimer(5 * time.Millisecond)
			defer timer.Stop()
			url := fmt.Sprintf("http://localhost:%d/jasper/v1/", port)
			for trials := 0; trials < 10; trials++ {
				timer.Reset(5 * time.Millisecond)
				select {
				case <-ctx.Done():
					return nil, -1, ctx.Err()
				case <-timer.C:
					req, err := http.NewRequest(http.MethodGet, url, nil)
					if err != nil {
						continue
					}
					rctx, cancel := context.WithTimeout(ctx, time.Second)
					defer cancel()
					req = req.WithContext(rctx)

					resp, err := client.Do(req)
					if err != nil {
						continue
					}
					if resp.StatusCode != http.StatusOK {
						continue
					}

					return srv, port, nil
				}
			}
		}
	}
}
