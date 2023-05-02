package jasper_test

import (
	"context"
	"testing"

	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testcases"
	"github.com/tychoish/jasper/testutil"
)

func TestManagerInterface(t *testing.T) {
	t.Parallel()

	for mname, makeMngr := range map[string]func(context.Context, *testing.T) jasper.Manager{
		"Basic/Lock": func(_ context.Context, t *testing.T) jasper.Manager {
			return jasper.NewManager(jasper.ManagerOptions{Synchronized: true})
		},
		"SelfClearing/NoLock": func(_ context.Context, t *testing.T) jasper.Manager {
			return jasper.NewManager(jasper.ManagerOptions{MaxProcs: 10})
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
