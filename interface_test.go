package jasper_test

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/testcases"
	"github.com/tychoish/jasper/testutil"
)

func TestManagerInterface(t *testing.T) {
	t.Parallel()

	for mname, makeMngr := range map[string]func(context.Context, *testing.T) jasper.Manager{
		"Basic/Lock": func(_ context.Context, t *testing.T) jasper.Manager {
			synchronizedManager, err := jasper.NewSynchronizedManager(false)
			assert.NotError(t, err)
			return synchronizedManager
		},
		"SelfClearing/NoLock": func(_ context.Context, t *testing.T) jasper.Manager {
			selfClearingManager, err := jasper.NewSelfClearingProcessManager(10, false)
			assert.NotError(t, err)
			return selfClearingManager
		},
		"Remote/NoLock/NilOptions": func(_ context.Context, t *testing.T) jasper.Manager {
			m, err := jasper.NewBasicProcessManager(false, false)
			assert.NotError(t, err)
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
