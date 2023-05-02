package jasper

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper/options"
)

func TestLogging(t *testing.T) {
	for _, test := range []struct {
		Name string
		Case func(*testing.T, LoggingCache)
	}{
		{
			Name: "Fixture",
			Case: func(t *testing.T, cache LoggingCache) {
				check.Equal(t, 0, cache.Len())
			},
		},
		{
			Name: "SafeOps",
			Case: func(t *testing.T, cache LoggingCache) {
				cache.Remove("")
				cache.Remove("what")
				check.Zero(t, cache.Get("whatever"))
				check.Error(t, cache.Put("foo", nil))
				check.Equal(t, 0, cache.Len())
			},
		},
		{
			Name: "PutGet",
			Case: func(t *testing.T, cache LoggingCache) {
				check.NotError(t, cache.Put("id", &options.CachedLogger{ID: "id"}))
				check.Equal(t, 1, cache.Len())
				lg := cache.Get("id")
				assert.True(t, lg != nil)
				check.Equal(t, "id", lg.ID)
			},
		},
		{
			Name: "PutDuplicate",
			Case: func(t *testing.T, cache LoggingCache) {
				check.NotError(t, cache.Put("id", &options.CachedLogger{ID: "id"}))
				check.Error(t, cache.Put("id", &options.CachedLogger{ID: "id"}))
				check.Equal(t, 1, cache.Len())
			},
		},
		{
			Name: "Prune",
			Case: func(t *testing.T, cache LoggingCache) {
				check.NotError(t, cache.Put("id", &options.CachedLogger{ID: "id"}))
				check.Equal(t, 1, cache.Len())
				cache.Prune(time.Now().Add(-time.Minute))
				check.Equal(t, 1, cache.Len())
				cache.Prune(time.Now().Add(time.Minute))
				check.Equal(t, 0, cache.Len())
			},
		},
		{
			Name: "CreateDuplicateProtection",
			Case: func(t *testing.T, cache LoggingCache) {
				cl, err := cache.Create("id", &options.Output{})
				assert.NotError(t, err)
				assert.True(t, cl != nil)

				cl, err = cache.Create("id", &options.Output{})
				assert.Error(t, err)
				assert.True(t, cl == nil)
			},
		},
		{
			Name: "CreateAccessTime",
			Case: func(t *testing.T, cache LoggingCache) {
				cl, err := cache.Create("id", &options.Output{})
				assert.NotError(t, err)

				check.True(t, time.Since(cl.Accessed) <= time.Millisecond)
			},
		},
		{
			Name: "CloseAndRemove",
			Case: func(t *testing.T, cache LoggingCache) {
				ctx := context.TODO()
				sender := options.NewMockSender("output")

				assert.NotError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender,
				}))
				assert.NotError(t, cache.Put("id1", &options.CachedLogger{}))
				assert.True(t, cache.Get("id0") != nil)
				assert.NotError(t, cache.CloseAndRemove(ctx, "id0"))
				assert.True(t, cache.Get("id0") == nil)
				check.NotZero(t, cache.Get("id1"))
				assert.True(t, sender.Closed)

				assert.NotError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender,
				}))
				assert.True(t, cache.Get("id0") != nil)
				check.Error(t, cache.CloseAndRemove(ctx, "id0"))
				assert.True(t, cache.Get("id0") == nil)
			},
		},
		{
			Name: "Clear",
			Case: func(t *testing.T, cache LoggingCache) {
				ctx := context.TODO()
				sender0 := options.NewMockSender("output")
				sender1 := options.NewMockSender("output")

				assert.NotError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender0,
				}))
				assert.NotError(t, cache.Put("id1", &options.CachedLogger{
					Output: sender1,
				}))
				assert.True(t, cache.Get("id0") != nil)
				assert.True(t, cache.Get("id1") != nil)
				assert.NotError(t, cache.Clear(ctx))
				assert.True(t, cache.Get("id0") == nil)
				assert.True(t, cache.Get("id1") == nil)
				assert.True(t, sender0.Closed)
				assert.True(t, sender1.Closed)

				assert.NotError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender0,
				}))
				assert.NotError(t, cache.Put("id1", &options.CachedLogger{
					Output: sender1,
				}))
				assert.True(t, cache.Get("id0") != nil)
				assert.True(t, cache.Get("id1") != nil)
				_ = cache.Clear(ctx)
				check.Zero(t, cache.Get("id0"))
				check.Zero(t, cache.Get("id1"))
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert.NotPanic(t, func() {
				test.Case(t, NewLoggingCache())
			})
		})
	}
}
