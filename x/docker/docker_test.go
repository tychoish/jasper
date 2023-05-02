package docker

import (
	"context"
	"os"
	"testing"

	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testcases"
	"github.com/tychoish/jasper/testutil"
)

func TestManagerInterface(t *testing.T) {
	t.Parallel()

	for mname, makeMngr := range map[string]func(context.Context, *testing.T) jasper.Manager{
		"Docker/NoLock": func(_ context.Context, t *testing.T) jasper.Manager {
			m := jasper.NewManager(jasper.ManagerOptions{})

			image := os.Getenv("DOCKER_IMAGE")
			if image == "" {
				image = testutil.DefaultDockerImage
			}
			return NewDockerManager(m, &options.Docker{
				Image: image,
			})
		},
	} {
		if testutil.IsDockerCase(mname) {
			testutil.SkipDockerIfUnsupported(t)
		}

		t.Run(mname, func(t *testing.T) {
			testcases.RunManagerSuite(t, testcases.GenerateManagerSuite(t), makeMngr)
		})
	}
}
