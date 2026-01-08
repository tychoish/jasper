package jasper

import (
	"context"
	"maps"

	"github.com/google/uuid"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
	"github.com/tychoish/jasper/options"
)

type ManagerOptions struct {
	ID           string
	MaxProcs     int
	Synchronized bool

	Tracker          ProcessTracker
	Remote           *options.Remote
	EnvVars          *dt.List[irt.KV[string, string]]
	ExecutorResolver func(context.Context, *options.Create) options.ResolveExecutor
}

func (conf *ManagerOptions) Validate() error {
	if conf.EnvVars == nil {conf.EnvVars = new(dt.List[irt.KV[string,string]])
	}

	if conf.ID == "" {
		conf.ID = uuid.New().String()
	}

	if conf.EnvVars.Len() == 0 {
		conf.EnvVars.PushBack(irt.MakeKV(ManagerEnvironID, conf.ID))
	} else {
		for p := conf.EnvVars.Front(); p.Ok(); p = p.Next() {
			if p.Value().Key == ManagerEnvironID {
				p.Set(irt.MakeKV(ManagerEnvironID, conf.ID))
				return nil
			}
		}
		conf.EnvVars.PushBack(irt.MakeKV(ManagerEnvironID, conf.ID))
	}
	return nil
}

type ManagerOptionProvider = opt.Provider[*ManagerOptions]

func ManagerOptionDefaults() ManagerOptionProvider {
	return func(conf *ManagerOptions) error {
		conf.Synchronized = true
		conf.MaxProcs = 1024
		return nil
	}
}

func ManagerOptionWithEnvVar(name, value string) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.EnvVars.PushBack(irt.MakeKV(name, value)); return nil }
}

func ManagerOptionWithEnvVarMap(mp map[string]string) ManagerOptionProvider {
	return func(conf *ManagerOptions) error { conf.EnvVars.Extend(irt.KVjoin(maps.All(mp))); return nil }
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
