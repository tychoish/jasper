package jasper

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func TestDefaultTrigger(t *testing.T) {
	const parentID = "parent-trigger-id"

	for name, testcase := range map[string]func(context.Context, *testing.T, Manager){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, manager Manager) {
			assert.True(t, manager != nil)
			assert.True(t, ctx != nil)
			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, len(out), 0)
			check.True(t, MakeDefaultTrigger(ctx, manager, testutil.TrueCreateOpts(), parentID) != nil)
			check.True(t, MakeDefaultTrigger(ctx, manager, nil, "") != nil)
		},
		"OneOnFailure": func(ctx context.Context, t *testing.T, manager Manager) {
			opts := testutil.FalseCreateOpts()
			tcmd := testutil.TrueCreateOpts()
			opts.OnFailure = append(opts.OnFailure, tcmd)
			trigger := MakeDefaultTrigger(ctx, manager, opts, parentID)
			trigger(ProcessInfo{})

			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			assert.Equal(t, len(out), 1)
			_, err = out[0].Wait(ctx)
			assert.NotError(t, err)
			info := out[0].Info(ctx)
			check.True(t, info.IsRunning || info.Complete)
		},
		"OneOnSuccess": func(ctx context.Context, t *testing.T, manager Manager) {
			opts := testutil.TrueCreateOpts()
			tcmd := testutil.FalseCreateOpts()
			opts.OnSuccess = append(opts.OnSuccess, tcmd)
			trigger := MakeDefaultTrigger(ctx, manager, opts, parentID)
			trigger(ProcessInfo{Successful: true})

			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			assert.Equal(t, len(out), 1)
			info := out[0].Info(ctx)
			check.True(t, info.IsRunning || info.Complete)
		},
		"FailureTriggerDoesNotWorkWithCanceledContext": func(ctx context.Context, t *testing.T, manager Manager) {
			cctx, cancel := context.WithCancel(ctx)
			cancel()
			opts := testutil.FalseCreateOpts()
			tcmd := testutil.TrueCreateOpts()
			opts.OnFailure = append(opts.OnFailure, tcmd)
			trigger := MakeDefaultTrigger(cctx, manager, opts, parentID)
			trigger(ProcessInfo{})

			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, 0, len(out))
		},
		"SuccessTriggerDoesNotWorkWithCanceledContext": func(ctx context.Context, t *testing.T, manager Manager) {
			cctx, cancel := context.WithCancel(ctx)
			cancel()
			opts := testutil.FalseCreateOpts()
			tcmd := testutil.TrueCreateOpts()
			opts.OnSuccess = append(opts.OnSuccess, tcmd)
			trigger := MakeDefaultTrigger(cctx, manager, opts, parentID)
			trigger(ProcessInfo{Successful: true})

			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, 0, len(out))
		},
		"SuccessOutcomeWithNoTriggers": func(ctx context.Context, t *testing.T, manager Manager) {
			trigger := MakeDefaultTrigger(ctx, manager, testutil.TrueCreateOpts(), parentID)
			trigger(ProcessInfo{})
			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, 0, len(out))
		},
		"FailureOutcomeWithNoTriggers": func(ctx context.Context, t *testing.T, manager Manager) {
			trigger := MakeDefaultTrigger(ctx, manager, testutil.TrueCreateOpts(), parentID)
			trigger(ProcessInfo{Successful: true})
			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, 0, len(out))
		},
		"TimeoutWithTimeout": func(ctx context.Context, t *testing.T, manager Manager) {
			opts := testutil.FalseCreateOpts()
			tcmd := testutil.TrueCreateOpts()
			opts.OnTimeout = append(opts.OnTimeout, tcmd)

			tctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			trigger := MakeDefaultTrigger(tctx, manager, opts, parentID)
			trigger(ProcessInfo{Timeout: true})

			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			assert.Equal(t, len(out), 1)
			_, err = out[0].Wait(ctx)
			check.NotError(t, err)
			info := out[0].Info(ctx)
			check.True(t, info.IsRunning || info.Complete)
		},
		"TimeoutWithoutTimeout": func(ctx context.Context, t *testing.T, manager Manager) {
			opts := testutil.FalseCreateOpts()
			tcmd := testutil.TrueCreateOpts()
			opts.OnTimeout = append(opts.OnTimeout, tcmd)

			trigger := MakeDefaultTrigger(ctx, manager, opts, parentID)
			trigger(ProcessInfo{Timeout: true})

			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			assert.Equal(t, len(out), 1)
			_, err = out[0].Wait(ctx)
			check.NotError(t, err)
			info := out[0].Info(ctx)
			check.True(t, info.IsRunning || info.Complete)
		},
		"TimeoutWithCanceledContext": func(ctx context.Context, t *testing.T, manager Manager) {
			cctx, cancel := context.WithCancel(ctx)
			cancel()

			opts := testutil.FalseCreateOpts()
			tcmd := testutil.TrueCreateOpts()
			opts.OnTimeout = append(opts.OnTimeout, tcmd)

			trigger := MakeDefaultTrigger(cctx, manager, opts, parentID)
			trigger(ProcessInfo{Timeout: true})

			out, err := manager.List(ctx, options.All)
			assert.NotError(t, err)
			check.Equal(t, 0, len(out))
		},
		"OptionsCloseTriggerCallsClosers": func(ctx context.Context, t *testing.T, manager Manager) {
			count := 0
			opts := options.Create{}
			opts.RegisterCloser(func() (_ error) { count++; return })
			info := ProcessInfo{Options: opts}

			trigger := makeOptionsCloseTrigger()
			trigger(info)
			check.Equal(t, 1, count)
		},
		// "": func(ctx context.Context, t *testing.T, manager Manager) {},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			testcase(ctx, t, &synchronizedProcessManager{
				manager: &basicProcessManager{
					loggers: NewLoggingCache(),
					procs:   map[string]Process{},
				},
			})
		})
	}
}
