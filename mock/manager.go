package mock

import (
	"context"
	"fmt"
	"runtime"

	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
)

// Manager implements the Manager interface with exported fields to
// configure and introspect the mock's behavior.
type Manager struct {
	FailCreate      bool
	FailRegister    bool
	FailList        bool
	FailGroup       bool
	FailGet         bool
	FailClose       bool
	NilLoggingCache bool
	FailWriteFile   bool
	Create          func(*options.Create) Process
	CreateConfig    Process
	ManagerID       string
	Procs           []jasper.Process
	ScriptingEnv    scripting.Harness
	LoggingCacheVal jasper.LoggingCache

	// WriteFile input
	WriteFileOptions options.WriteFile
}

func mockFail() error {
	progCounter := make([]uintptr, 2)
	n := runtime.Callers(2, progCounter)
	frames := runtime.CallersFrames(progCounter[:n])
	frame, _ := frames.Next()
	return fmt.Errorf("function failed: %s", frame.Function)
}

// ID returns the ManagerID field.
func (m *Manager) ID() string {
	return m.ManagerID
}

// CreateProcess creates a new mock Process. If Create is set, it is
// invoked to create the mock Process. Otherwise, CreateConfig is used as a
// template to create the mock Process. The new mock Process is put in Procs. If
// FailCreate is set, it returns an error.
func (m *Manager) CreateProcess(ctx context.Context, opts *options.Create) (jasper.Process, error) {
	if m.FailCreate {
		return nil, mockFail()
	}

	var proc Process
	if m.Create != nil {
		proc = m.Create(opts)
	} else {
		proc = m.CreateConfig
		proc.ProcInfo.Options = *opts
	}

	m.Procs = append(m.Procs, &proc)

	return &proc, nil
}

// CreateCommand creates a Command that invokes CreateProcess to create the
// underlying processes.
func (m *Manager) CreateCommand(ctx context.Context) *jasper.Command {
	return jasper.NewCommand().ProcConstructor(m.CreateProcess)
}

// LoggingCache returns the implementation's logging cache.
func (m *Manager) LoggingCache(ctx context.Context) jasper.LoggingCache {
	if m.NilLoggingCache {
		return nil
	}
	return m.LoggingCacheVal
}

// Register adds the process to Procs. If FailRegister is set, it returns an
// error.
func (m *Manager) Register(ctx context.Context, proc jasper.Process) error {
	if m.FailRegister {
		return mockFail()
	}

	m.Procs = append(m.Procs, proc)

	return nil
}

// List returns all processes that match the given filter. If FailList is set,
// it returns an error.
func (m *Manager) List(ctx context.Context, f options.Filter) ([]jasper.Process, error) {
	if m.FailList {
		return nil, mockFail()
	}

	filteredProcs := []jasper.Process{}

	for _, proc := range m.Procs {
		info := proc.Info(ctx)
		switch f {
		case options.All:
			filteredProcs = append(filteredProcs, proc)
		case options.Running:
			if info.IsRunning {
				filteredProcs = append(filteredProcs, proc)
			}
		case options.Terminated:
			if !info.IsRunning {
				filteredProcs = append(filteredProcs, proc)
			}
		case options.Failed:
			if info.Complete && !info.Successful {
				filteredProcs = append(filteredProcs, proc)
			}
		case options.Successful:
			if info.Successful {
				filteredProcs = append(filteredProcs, proc)
			}
		default:
			return nil, fmt.Errorf("invalid filter '%s'", f)
		}
	}

	return filteredProcs, nil
}

// Group returns all processses that have the given tag. If FailGroup is set, it
// returns an error.
func (m *Manager) Group(ctx context.Context, tag string) ([]jasper.Process, error) {
	if m.FailGroup {
		return nil, mockFail()
	}

	matchingProcs := []jasper.Process{}
	for _, proc := range m.Procs {
		for _, procTag := range proc.GetTags() {
			if procTag == tag {
				matchingProcs = append(matchingProcs, proc)
			}
		}
	}

	return matchingProcs, nil
}

// Get returns a process given by ID from Procs. If a matching process is not
// found in Procs or if FailGet is set, it returns an error.
func (m *Manager) Get(ctx context.Context, id string) (jasper.Process, error) {
	if m.FailGet {
		return nil, mockFail()
	}

	for _, proc := range m.Procs {
		if proc.ID() == id {
			return proc, nil
		}
	}

	return nil, fmt.Errorf("proc with id '%s' not found", id)
}

// Clear removes all processes from Procs.
func (m *Manager) Clear(ctx context.Context) {
	m.Procs = []jasper.Process{}
}

// Close clears all processes in Procs. If FailClose is set, it returns an
// error.
func (m *Manager) Close(ctx context.Context) error {
	if m.FailClose {
		return mockFail()
	}
	m.Clear(ctx)
	return nil
}

func (m *Manager) WriteFile(ctx context.Context, opts options.WriteFile) error {
	if m.FailWriteFile {
		return mockFail()
	}
	m.WriteFileOptions = opts
	return nil
}
