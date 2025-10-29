package jasper

import (
	"context"

	"github.com/google/uuid"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/jasper/options"
)

type ManagerOptions struct {
	ID           string
	MaxProcs     int
	Synchronized bool

	Tracker          ProcessTracker
	Remote           *options.Remote
	EnvVars          *dt.List[dt.Pair[string, string]]
	ExecutorResolver func(context.Context, *options.Create) options.ResolveExecutor
}

func (conf *ManagerOptions) Validate() error {
	conf.EnvVars = ft.DefaultNew(conf.EnvVars)

	if conf.ID == "" {
		conf.ID = uuid.New().String()
	}

	if conf.EnvVars.Len() == 0 {
		conf.EnvVars.PushBack(dt.MakePair(ManagerEnvironID, conf.ID))
	} else {
		for p := conf.EnvVars.Front(); p.Ok(); p = p.Next() {
			if p.Value().Key == ManagerEnvironID {
				p.Set(dt.MakePair(ManagerEnvironID, conf.ID))
				return nil
			}
		}
		conf.EnvVars.PushBack(dt.MakePair(ManagerEnvironID, conf.ID))
	}
	return nil
}

type ManagerOptionProvider = fun.OptionProvider[*ManagerOptions]

func ManagerOptionDefaults() ManagerOptionProvider {
	return func(conf *ManagerOptions) error {
		conf.Synchronized = true
		conf.MaxProcs = 1024
		return nil
	}
}

func ManagerOptionWithEnvVar(name, value string) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.EnvVars.PushBack(dt.MakePair(name, value)); return nil }
}

func ManagerOptionWithEnvVarMap(mp map[string]string) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { return conf.EnvVars.AppendStream(dt.NewMap(mp).Stream()).Wait() }
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
