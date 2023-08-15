package options

import (
	"errors"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

type testPostHook struct {
	count    int
	abort    bool
	errsSeen int
}

func (t *testPostHook) hook(err error) error {
	t.count++

	if err != nil {
		t.errsSeen++
		if t.abort {
			return err
		}
	}
	return nil
}

func TestCommand(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		t.Run("InvalidByDefault", func(t *testing.T) {
			assert.Error(t, (&Command{}).Validate())
		})
		t.Run("ValidatePopulatesProcessArgs", func(t *testing.T) {
			opts := &Command{}
			check.True(t, opts.Process.Args == nil)
			assert.Error(t, opts.Validate())
			check.True(t, opts.Process.Args != nil)
			assert.Equal(t, len(opts.Process.Args), 1)
			assert.Equal(t, "", opts.Process.Args[0])
		})
		t.Run("Valid", func(t *testing.T) {
			opts := &Command{
				Priority: level.Info,
				Commands: [][]string{{""}},
			}
			check.NotError(t, opts.Validate())
		})
	})
	t.Run("LoggingPreHook", func(t *testing.T) {
		sender := send.NewInternal(10)
		sender.SetPriority(level.Debug)
		sender.SetName("pre-hook")
		logger := grip.NewLogger(sender)
		hook := NewLoggingPreHook(logger, level.Info)
		check.True(t, hook != nil)
		cmd := &Command{ID: "TEST"}
		check.True(t, !sender.HasMessage())
		hook(cmd, &Create{})
		assert.True(t, sender.HasMessage())
		check.Equal(t, 1, sender.Len())
		msg, ok := sender.GetMessage().Message.Raw().(message.Fields)
		assert.True(t, ok)
		check.Equal(t, cmd.ID, msg["id"].(string))
	})
	t.Run("PrehookConstrcutors", func(t *testing.T) {
		check.True(t, NewDefaultLoggingPreHook(level.Info) != nil)
		check.True(t, NewLoggingPreHookFromSender(grip.Sender(), level.Debug) != nil)
	})
	t.Run("MergePreook", func(t *testing.T) {
		sender := send.NewInternal(10)
		sender.SetPriority(level.Debug)
		sender.SetName("pre-hook")
		logger := grip.NewLogger(sender)

		hook := MergePreHooks(NewLoggingPreHook(logger, level.Info), NewLoggingPreHook(logger, level.Info), NewLoggingPreHook(logger, level.Info))
		check.Equal(t, 0, sender.Len())
		hook(&Command{}, &Create{})
		check.Equal(t, 3, sender.Len())
	})
	t.Run("MergePostHook", func(t *testing.T) {
		t.Run("Harness", func(t *testing.T) {
			t.Run("Counter", func(t *testing.T) {
				mock := &testPostHook{}
				check.Equal(t, 0, mock.count)
				check.NotError(t, mock.hook(nil))
				check.Equal(t, 1, mock.count)
			})
			t.Run("Abort", func(t *testing.T) {
				mock := &testPostHook{abort: true}
				check.NotError(t, mock.hook(nil))
				check.Equal(t, 1, mock.count)
				check.Equal(t, 0, mock.errsSeen)

				check.Error(t, mock.hook(errors.New("hi")))
				check.Equal(t, 2, mock.count)
				check.Equal(t, 1, mock.errsSeen)
			})
		})
		t.Run("Merged", func(t *testing.T) {
			mock := &testPostHook{abort: true}
			hook := MergePostHooks(mock.hook, mock.hook)
			check.NotError(t, hook(nil))
			check.Equal(t, 2, mock.count)
			check.Error(t, hook(errors.New("hi")))
			check.Equal(t, 4, mock.count)
			check.Equal(t, 2, mock.errsSeen)
		})
		t.Run("ShortCircuit", func(t *testing.T) {
			mock := &testPostHook{abort: true}
			hook := MergeAbortingPostHooks(mock.hook, mock.hook)
			check.NotError(t, hook(nil))
			check.Equal(t, 2, mock.count)

			check.Error(t, hook(errors.New("hi")))
			check.Equal(t, 3, mock.count)
			check.Equal(t, 1, mock.errsSeen)
		})
		t.Run("Passthrough", func(t *testing.T) {
			mock := &testPostHook{}
			hook := MergeAbortingPassthroughPostHooks(mock.hook, mock.hook)
			check.NotError(t, hook(nil))
			check.Equal(t, 2, mock.count)
			check.Equal(t, 0, mock.errsSeen)

			err := errors.New("hi")
			check.True(t, err == hook(err))
			check.Equal(t, 4, mock.count)
			check.Equal(t, 2, mock.errsSeen)

			mock.abort = true
			check.True(t, err == hook(err))
			check.Equal(t, 5, mock.count)
			check.Equal(t, 3, mock.errsSeen)
		})
	})
}
