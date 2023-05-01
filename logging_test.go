package jasper

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
				require.NotNil(t, lg)
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
				require.NoError(t, err)
				require.NotNil(t, cl)

				cl, err = cache.Create("id", &options.Output{})
				require.Error(t, err)
				require.Nil(t, cl)
			},
		},
		{
			Name: "CreateAccessTime",
			Case: func(t *testing.T, cache LoggingCache) {
				cl, err := cache.Create("id", &options.Output{})
				require.NoError(t, err)

				check.True(t, time.Since(cl.Accessed) <= time.Millisecond)
			},
		},
		{
			Name: "CloseAndRemove",
			Case: func(t *testing.T, cache LoggingCache) {
				ctx := context.TODO()
				sender := options.NewMockSender("output")

				require.NoError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender,
				}))
				require.NoError(t, cache.Put("id1", &options.CachedLogger{}))
				require.NotNil(t, cache.Get("id0"))
				require.NoError(t, cache.CloseAndRemove(ctx, "id0"))
				require.Nil(t, cache.Get("id0"))
				check.NotZero(t, cache.Get("id1"))
				require.True(t, sender.Closed)

				require.NoError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender,
				}))
				require.NotNil(t, cache.Get("id0"))
				check.Error(t, cache.CloseAndRemove(ctx, "id0"))
				require.Nil(t, cache.Get("id0"))
			},
		},
		{
			Name: "Clear",
			Case: func(t *testing.T, cache LoggingCache) {
				ctx := context.TODO()
				sender0 := options.NewMockSender("output")
				sender1 := options.NewMockSender("output")

				require.NoError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender0,
				}))
				require.NoError(t, cache.Put("id1", &options.CachedLogger{
					Output: sender1,
				}))
				require.NotNil(t, cache.Get("id0"))
				require.NotNil(t, cache.Get("id1"))
				require.NoError(t, cache.Clear(ctx))
				require.Nil(t, cache.Get("id0"))
				require.Nil(t, cache.Get("id1"))
				require.True(t, sender0.Closed)
				require.True(t, sender1.Closed)

				require.NoError(t, cache.Put("id0", &options.CachedLogger{
					Output: sender0,
				}))
				require.NoError(t, cache.Put("id1", &options.CachedLogger{
					Output: sender1,
				}))
				require.NotNil(t, cache.Get("id0"))
				require.NotNil(t, cache.Get("id1"))
				_ = cache.Clear(ctx)
				check.Zero(t, cache.Get("id0"))
				check.Zero(t, cache.Get("id1"))
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			require.NotPanics(t, func() {
				test.Case(t, NewLoggingCache())
			})
		})
	}
}
