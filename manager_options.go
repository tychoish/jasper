package jasper

import (
	"context"

	"github.com/google/uuid"

	"github.com/tychoish/fun"
	"github.com/tychoish/jasper/options"
)

type ManagerOptions struct {
	ID           string
	Tracker      ProcessTracker
	MaxProcs     int
	Synchronized bool

	Remote           *options.Remote
	ExecutorResolver func(context.Context, *options.Create) options.ResolveExecutor
}

type ManagerOptionProvider = fun.OptionProvider[*ManagerOptions]

func ManagerOptionDefaults() ManagerOptionProvider {
	return func(conf *ManagerOptions) error {
		conf.ID = uuid.New().String()
		conf.Synchronized = true
		conf.MaxProcs = 1024
		return nil
	}
}

func ManagerOptionSet(opts ManagerOptions) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { *conf = opts; return nil }
}

func ManagerOptionMaxProcs(n int) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.MaxProcs = n; return nil }
}

func ManagerOptionSetSynchronized() ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.Synchronized = true; return nil }
}

func ManagerOptionSetUnynchronized() ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.Synchronized = false; return nil }
}

func ManagerOptionWithRemote(opt *options.Remote) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.Remote = opt; return nil }
}

func ManagerOptionExecutorResolver(er func(context.Context, *options.Create) options.ResolveExecutor) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.ExecutorResolver = er; return nil }
}
