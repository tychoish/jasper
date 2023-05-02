package jasper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper/executor"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/util"
)

type ManagerOptions struct {
	ID           string
	Tracker      ProcessTracker
	Remote       *options.Remote
	MaxProcs     int
	SSHLibrary   bool
	Synchronized bool
}

func NewManager(opts ManagerOptions) Manager {
	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}

	var mgr Manager
	m := &basicProcessManager{
		procs:         map[string]Process{},
		id:            opts.ID,
		useSSHLibrary: opts.SSHLibrary,
		loggers:       NewLoggingCache(),
		tracker:       opts.Tracker,
	}
	mgr = m

	if opts.MaxProcs > 0 {
		mgr = &selfClearingProcessManager{
			basicProcessManager: m,
			maxProcs:            opts.MaxProcs,
		}
	}

	if opts.Remote != nil {
		mgr = &remoteOverrideMgr{remote: opts.Remote, Manager: mgr}
	}

	if opts.Synchronized {
		mgr = &synchronizedProcessManager{manager: m}
	}

	return mgr
}

type basicProcessManager struct {
	id            string
	procs         map[string]Process
	useSSHLibrary bool
	tracker       ProcessTracker
	loggers       LoggingCache
}

func (m *basicProcessManager) ID() string { return m.id }

func (m *basicProcessManager) CreateProcess(ctx context.Context, opts *options.Create) (Process, error) {
	opts.AddEnvVar(ManagerEnvironID, m.id)

	if opts.Remote != nil {
		if m.useSSHLibrary {
			client, session, err := opts.Remote.Resolve()
			if err != nil {
				return nil, fmt.Errorf("could not resolve SSH client and session: %w", err)
			}
			opts.ResolveExecutor = func(ctx context.Context, args []string) executor.Executor {
				return executor.NewSSH(ctx, client, session, args)
			}
			opts.Remote.UseSSHLibrary = true
		} else {
			opts.ResolveExecutor = func(ctx context.Context, args []string) executor.Executor {
				return executor.NewSSHBinary(ctx, opts.Remote.String(), opts.Remote.Args, args)
			}
		}
	}

	proc, err := NewProcess(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("problem constructing process: %w", err)
	}

	grip.Warning(message.WrapError(m.loggers.Put(proc.ID(), &options.CachedLogger{
		ID:      proc.ID(),
		Manager: m.id,
		Error:   util.ConvertWriter(opts.Output.GetError()),
		Output:  util.ConvertWriter(opts.Output.GetOutput()),
	}), message.Fields{
		"message": "problem caching logger for process",
		"process": proc.ID(),
		"manager": m.ID(),
	}))

	// This trigger is not guaranteed to be registered since the process may
	// have already completed. One way to guarantee it runs could be to add this
	// as a closer to CreateOptions.
	_ = proc.RegisterTrigger(ctx, MakeDefaultTrigger(ctx, m, opts, proc.ID()))

	if m.tracker != nil {
		// The process may have terminated already, so don't return on error.
		if err := m.tracker.Add(proc.Info(ctx)); err != nil {
			grip.Warning(message.WrapError(err, "problem adding process to tracker during process creation"))
		}
	}

	m.procs[proc.ID()] = proc

	return proc, nil
}

func (m *basicProcessManager) LoggingCache(_ context.Context) LoggingCache { return m.loggers }

func (m *basicProcessManager) CreateCommand(ctx context.Context) *Command {
	return NewCommand().ProcConstructor(m.CreateProcess)
}

func (m *basicProcessManager) Register(ctx context.Context, proc Process) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if proc == nil {
		return errors.New("process is not defined")
	}

	id := proc.ID()
	if id == "" {
		return errors.New("process is malformed")
	}

	if m.tracker != nil {
		// The process may have terminated already, so don't return on error.
		if err := m.tracker.Add(proc.Info(ctx)); err != nil {
			grip.Warning(message.WrapError(err, "problem adding process to tracker during process registration"))
		}
	}

	_, ok := m.procs[id]
	if ok {
		return errors.New("cannot register process that exists")
	}

	m.procs[id] = proc
	return nil
}

func (m *basicProcessManager) List(ctx context.Context, f options.Filter) ([]Process, error) {
	out := []Process{}

	if err := f.Validate(); err != nil {
		return out, fmt.Errorf("invalid filter: %w", err)
	}

	for _, proc := range m.procs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		cctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		info := proc.Info(cctx)
		cancel()
		switch {
		case f == options.Running:
			if info.IsRunning {
				out = append(out, proc)
			}
		case f == options.Terminated:
			if !info.IsRunning {
				out = append(out, proc)
			}
		case f == options.Successful:
			if info.Successful {
				out = append(out, proc)
			}
		case f == options.Failed:
			if info.Complete && !info.Successful {
				out = append(out, proc)
			}
		case f == options.All:
			out = append(out, proc)
		}
	}

	return out, nil
}

func (m *basicProcessManager) Get(ctx context.Context, id string) (Process, error) {
	proc, ok := m.procs[id]
	if !ok {
		return nil, fmt.Errorf("process '%s' does not exist", id)
	}

	return proc, nil
}

func (m *basicProcessManager) Clear(ctx context.Context) {
	for procID, proc := range m.procs {
		if proc.Complete(ctx) {
			delete(m.procs, procID)
			m.loggers.Remove(procID)
		}
	}
}

func (m *basicProcessManager) Close(ctx context.Context) error {
	if len(m.procs) == 0 {
		return nil
	}
	procs, err := m.List(ctx, options.Running)
	if err != nil {
		return err
	}

	termCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if m.tracker != nil {
		if err := m.tracker.Cleanup(); err != nil {
			grip.Warning(message.WrapError(err, "process tracker did not clean up all processes successfully"))
		} else {
			return nil
		}
	}
	if err := TerminateAll(termCtx, procs); err != nil {
		killCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		return KillAll(killCtx, procs)
	}

	return nil
}

func (m *basicProcessManager) Group(ctx context.Context, name string) ([]Process, error) {
	out := []Process{}
	for _, proc := range m.procs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

	addTag:
		for _, t := range proc.GetTags() {
			if t == name {
				out = append(out, proc)
				break addTag
			}
		}
	}

	return out, nil
}

func (m *basicProcessManager) WriteFile(ctx context.Context, opts options.WriteFile) error {
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid write options: %w", err)
	}

	if err := opts.DoWrite(); err != nil {
		return fmt.Errorf("error writing file '%s': %w", opts.Path, err)
	}
	return nil
}
