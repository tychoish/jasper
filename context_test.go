package jasper

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr, err := NewSynchronizedManager(false)
	assert.NotError(t, err)

	check.True(t, !HasManager(ctx))

	ctx = WithManager(ctx, mgr)
	check.True(t, HasManager(ctx))

	mgr2 := Context(ctx)

	if mgr != mgr2 {
		t.Fatal("should be the same ")
	}

}
