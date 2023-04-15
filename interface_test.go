package jasper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testcases"
	"github.com/tychoish/jasper/testutil"
)

func TestManagerInterface(t *testing.T) {
	t.Parallel()

	for mname, makeMngr := range map[string]func(context.Context, *testing.T) jasper.Manager{
		// "Basic/NoLock": func(_ context.Context, _ *testing.T) jasper.Manager {
		// 	return &basicProcessManager{
		// 		id:      "id",
		// 		loggers: NewLoggingCache(),
		// 		procs:   map[string]jasper.Process{},
		// 	}
		// },
		"Basic/Lock": func(_ context.Context, t *testing.T) jasper.Manager {
			synchronizedManager, err := jasper.NewSynchronizedManager(false)
			require.NoError(t, err)
			return synchronizedManager
		},
		"SelfClearing/NoLock": func(_ context.Context, t *testing.T) jasper.Manager {
			selfClearingManager, err := jasper.NewSelfClearingProcessManager(10, false)
			require.NoError(t, err)
			return selfClearingManager
		},
		"Remote/NoLock/NilOptions": func(_ context.Context, t *testing.T) jasper.Manager {
			m, err := jasper.NewBasicProcessManager(false, false)
			require.NoError(t, err)
			return jasper.NewRemoteManager(m, nil)
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
