package jasper

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert/check"
)

func TestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := NewManager(ManagerOptions{})

	check.True(t, !HasManager(ctx))

	ctx = WithManager(ctx, mgr)
	check.True(t, HasManager(ctx))

	mgr2 := Context(ctx)

	if mgr != mgr2 {
		t.Fatal("should be the same ")
	}

	ctx = WithNewContextManager(ctx, string(defaultContextKey), func() Manager { panic("should not panic") })
	check.True(t, HasContextManager(ctx, string(defaultContextKey)))
	check.Panic(t, func() {
		ctx = WithNewContextManager(ctx, "novel-key", func() Manager { panic("should not panic") })
	})

}
