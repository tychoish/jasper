package scripting

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

type roswellEnvironment struct {
	opts *options.ScriptingRoswell

	isConfigured bool
	cachedHash   string
	manager      jasper.Manager
}

func (e *roswellEnvironment) ID() string { e.cachedHash = e.opts.ID(); return e.cachedHash }
func (e *roswellEnvironment) Setup(ctx context.Context) error {
	if e.isConfigured && e.cachedHash == e.opts.ID() {
		return nil
	}

	cmd := e.manager.CreateCommand(ctx).Environment(e.opts.Environment).AddEnv("ROSWELL_HOME", e.opts.Path).
		SetOutputOptions(e.opts.Output).AppendArgs(e.opts.Interpreter(), "install", e.opts.Lisp)
	for _, sys := range e.opts.Systems {
		cmd.AppendArgs(e.opts.Interpreter(), "install", sys)
	}

	cmd.PostHook(func(res error) error {
		if res == nil {
			e.isConfigured = true
		}
		return nil
	})

	return cmd.Run(ctx)
}

func (e *roswellEnvironment) Run(ctx context.Context, forms []string) error {
	ros := []string{
		e.opts.Interpreter(), "run",
	}
	for _, f := range forms {
		ros = append(ros, "-e", f)
	}
	ros = append(ros, "-q")

	return e.manager.CreateCommand(ctx).Environment(e.opts.Environment).AddEnv("ROSWELL_HOME", e.opts.Path).
		SetOutputOptions(e.opts.Output).Add(ros).Run(ctx)
}

func (e *roswellEnvironment) RunScript(ctx context.Context, script string) error {
	scriptChecksum := fmt.Sprintf("%x", sha1.Sum([]byte(script)))
	wo := options.WriteFile{
		Path:    filepath.Join(e.opts.Path, "tmp", strings.Join([]string{e.manager.ID(), scriptChecksum}, "-")+".ros"),
		Content: []byte(script),
	}

	if err := e.manager.WriteFile(ctx, wo); err != nil {
		return fmt.Errorf("problem writing script file: %w", err)
	}

	return e.manager.CreateCommand(ctx).Environment(e.opts.Environment).AddEnv("ROSWELL_HOME", e.opts.Path).
		SetOutputOptions(e.opts.Output).AppendArgs(e.opts.Interpreter(), wo.Path).Run(ctx)
}

func (e *roswellEnvironment) Build(ctx context.Context, dir string, args []string) (string, error) {
	err := e.manager.CreateCommand(ctx).Directory(dir).Environment(e.opts.Environment).AddEnv("ROSWELL_HOME", e.opts.Path).
		SetOutputOptions(e.opts.Output).Add(append([]string{e.opts.Interpreter(), "dump", "executable"}, args...)).Run(ctx)
	if err != nil {
		return "", err
	}

	if len(args) >= 1 {
		return strings.TrimRight(args[0], ".ros"), nil
	}

	return "", nil
}

func (e *roswellEnvironment) Cleanup(ctx context.Context) error {
	switch mgr := e.manager.(type) {
	case remote:
		if err := mgr.CreateCommand(ctx).SetOutputOptions(e.opts.Output).AppendArgs("rm", "-rf", e.opts.Path).Run(ctx); err != nil {
			return fmt.Errorf("problem removing remote roswell environment '%s': %w", e.opts.Path, err)
		}
	default:
		if err := os.RemoveAll(e.opts.Path); err != nil {
			return fmt.Errorf("problem removing local roswell environment '%s'", e.opts.Path)
		}
	}

	return nil
}

func (e *roswellEnvironment) Test(ctx context.Context, dir string, tests ...TestOptions) ([]TestResult, error) {
	out := make([]TestResult, len(tests))

	catcher := &erc.Collector{}
	for idx, t := range tests {
		if t.Count == 0 {
			t.Count++
		}
		startAt := time.Now()

		var (
			cancel context.CancelFunc
			tctx   context.Context
		)
		if t.Timeout > 0 {
			tctx, cancel = context.WithTimeout(ctx, t.Timeout)
		} else {
			tctx, cancel = context.WithCancel(ctx)
		}

		cmd := e.manager.CreateCommand(ctx).Directory(dir).Environment(e.opts.Environment).AddEnv("ROSWELL_HOME", e.opts.Path).SetOutputOptions(e.opts.Output).
			Add([]string{e.opts.Interpreter(), "install", t.Name})

		for i := 0; i < t.Count; i++ {
			cmd.Add(append(append([]string{e.opts.Interpreter(), "run", "-e", fmt.Sprintf("'(asdf:test-system :%s)'", t.Name)}, t.Args...), "-q"))
		}

		err := cmd.Run(tctx)
		if err != nil {
			catcher.Add(fmt.Errorf("roswell test %q: %w", t, err))
		}

		out[idx] = t.getResult(ctx, err, startAt)
		cancel()
	}

	return out, catcher.Resolve()
}
