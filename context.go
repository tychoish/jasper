package jasper

import (
	"context"

	"github.com/tychoish/fun"
)

type ctxKey string

const defaultContextKey ctxKey = "__JASPER_STD_MANAGER"

// WithManager attaches a Manager instance to the context
func WithManager(ctx context.Context, mgr Manager) context.Context {
	return WithContextManager(ctx, string(defaultContextKey), mgr)
}

// Context resolves a jasper.Manager from the given context, and if one does
// not exist (or the context is nil), produces the global Manager
// instance.
func Context(ctx context.Context) Manager { return ContextManager(ctx, string(defaultContextKey)) }

// WithContextManager attaches a jasper.Manager with a specific name
// to the context.
func WithContextManager(ctx context.Context, name string, mgr Manager) context.Context {
	return context.WithValue(ctx, ctxKey(name), mgr)
}

// WithNewContextLogger checks if a logger is configured with a
// specific name in the current context. If this logger exists,
// WithNewContextLogger is a noop; otherwise, it constructs a logger
// with the sender produced by the provided function and attaches it
// to the context returning that context.
//
// The name provided controls the id of the logger in the context, not
// the name of the logger.
func WithNewContextManager(ctx context.Context, name string, fn func() Manager) context.Context {
	if HasContextManager(ctx, name) {
		return ctx
	}

	return WithContextManager(ctx, name, fn())
}

// ContextLoger produces a jasper.Manager stored in the context by a given
// name. If such a context is not stored the standard/default jasper.Manager
// is returned.
func ContextManager(ctx context.Context, name string) Manager {
	val := ctx.Value(ctxKey(name))
	fun.Invariant.Ok(val != nil, "jasper", name, "manager must be stored")

	mgr, ok := val.(Manager)
	fun.Invariant.Ok(ok, "stored jasper manager", name, "must be of the correct type")

	return mgr
}

// HasContextManager checks the provided context to see if a Manager
// with the given name is attached to the provided context.
func HasContextManager(ctx context.Context, name string) bool {
	_, ok := ctx.Value(ctxKey(name)).(Manager)
	return ok
}

// HasManager returns true when the default context Manager is
// attached.
func HasManager(ctx context.Context) bool { return HasContextManager(ctx, string(defaultContextKey)) }
