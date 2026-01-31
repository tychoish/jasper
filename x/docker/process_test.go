package docker_test

import (
	"context"
	"os"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testcases"
	"github.com/tychoish/jasper/testutil"
)

func TestProcessImplementations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for pname, makeProc := range testcases.ProcessConstructors() {
		t.Run(pname, func(t *testing.T) {
			for optsTestName, modifyOpts := range map[string]func(*options.Create) *options.Create{
				"Docker": func(opts *options.Create) *options.Create {
					image := os.Getenv("DOCKER_IMAGE")
					if image == "" {
						image = testutil.DefaultDockerImage
					}
					opts.Docker = &options.Docker{
						Image: image,
					}
					return opts
				},
			} {
				if testutil.IsDockerCase(optsTestName) {
					testutil.SkipDockerIfUnsupported(t)
					// TODO (MAKE-1300): remove these lines that clean up docker
					// containers and replace with (Process).Close().
					defer func() {
						client, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
						assert.NotError(t, err)
						containers, err := client.ContainerList(ctx, container.ListOptions{All: true})
						assert.NotError(t, err)
						for _, ct := range containers {
							grip.Error(message.WrapError(client.ContainerRemove(ctx, ct.ID, container.RemoveOptions{Force: true}), "problem cleaning up container"))
						}
					}()
				}

				t.Run(optsTestName, func(t *testing.T) {
					for testName, testCase := range testcases.ProcessCases() {
						t.Run(testName, func(t *testing.T) {
							tctx, cancel := context.WithTimeout(ctx, testutil.ProcessTestTimeout)
							defer cancel()

							opts := &options.Create{Args: []string{"ls"}}
							opts = modifyOpts(opts)
							testCase(tctx, t, opts, makeProc)
						})
					}
				})
			}
		})
	}
}
