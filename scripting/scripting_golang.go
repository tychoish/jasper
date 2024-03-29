package scripting

import (
	"context"
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

type golangEnvironment struct {
	opts *options.ScriptingGolang

	isConfigured bool
	cachedHash   string
	manager      jasper.Manager
}

func (e *golangEnvironment) ID() string { e.cachedHash = e.opts.ID(); return e.cachedHash }
func (e *golangEnvironment) Setup(ctx context.Context) error {
	if e.isConfigured && e.cachedHash == e.opts.ID() {
		return nil
	}

	e.cachedHash = e.opts.ID()

	gobin := e.opts.Interpreter()
	cmd := e.manager.CreateCommand(ctx).
		Environment(e.opts.Environment).
		AddEnv("GOPATH", e.opts.Gopath).
		AddEnv("GOROOT", e.opts.Goroot)

	for _, pkg := range e.opts.Packages {
		if e.opts.UpdatePackages {
			cmd.AppendArgs(gobin, "get", "-u", pkg)
		} else {
			cmd.AppendArgs(gobin, "get", pkg)
		}
	}

	cmd.PostHook(func(res error) error {
		if res == nil {
			e.isConfigured = true
		}
		return nil
	})

	return cmd.SetOutputOptions(e.opts.Output).Run(ctx)
}

func (e *golangEnvironment) Run(ctx context.Context, args []string) error {
	cmd := e.manager.CreateCommand(ctx).
		Environment(e.opts.Environment).
		AddEnv("GOPATH", e.opts.Gopath).
		AddEnv("GOROOT", e.opts.Goroot).
		SetOutputOptions(e.opts.Output).
		Add(append([]string{e.opts.Interpreter(), "run"}, args...))

	if e.opts.Directory != "" {
		cmd.Directory(e.opts.Directory)
	}

	return cmd.Run(ctx)
}

func (e *golangEnvironment) Build(ctx context.Context, dir string, args []string) (string, error) {
	if e.opts.Directory != "" && !strings.HasPrefix(dir, e.opts.Directory) {
		dir = filepath.Join(e.opts.Directory, dir)
	}
	err := e.manager.CreateCommand(ctx).
		Directory(dir).
		Environment(e.opts.Environment).
		AddEnv("GOPATH", e.opts.Gopath).
		AddEnv("GOROOT", e.opts.Goroot).
		SetOutputOptions(e.opts.Output).
		Add(append([]string{e.opts.Interpreter(), "build"}, args...)).Run(ctx)

	if err != nil {
		return "", err
	}

	for idx, val := range args {
		if val == "-o" {
			if len(args) >= idx+1 {
				return args[idx+1], nil
			}
			break
		}
	}

	return "", nil
}

func (e *golangEnvironment) RunScript(ctx context.Context, script string) error {
	scriptChecksum := fmt.Sprintf("%x", sha1.Sum([]byte(script)))
	path := strings.Join([]string{e.manager.ID(), scriptChecksum}, "_") + ".go"
	if e.opts.Directory != "" {
		path = filepath.Join(e.opts.Directory, path)
	} else {
		path = filepath.Join(e.opts.Gopath, "tmp", path)

	}
	wo := options.WriteFile{
		Path:    path,
		Content: []byte(script),
	}
	if err := e.manager.WriteFile(ctx, wo); err != nil {
		return fmt.Errorf("problem writing script file: %w", err)
	}

	return e.manager.CreateCommand(ctx).Environment(e.opts.Environment).
		SetOutputOptions(e.opts.Output).AppendArgs(e.opts.Interpreter(), "run", wo.Path).Run(ctx)
}

func (e *golangEnvironment) Cleanup(ctx context.Context) error {
	if err := e.manager.CreateCommand(ctx).SetOutputOptions(e.opts.Output).Sudo(true).AppendArgs("rm", "-rf", e.opts.Gopath).Run(ctx); err != nil {
		return fmt.Errorf("problem removing golang environment '%s': %w", e.opts.Gopath, err)
	}
	return nil
}

func (e *golangEnvironment) Test(ctx context.Context, dir string, tests ...TestOptions) ([]TestResult, error) {
	if e.opts.Directory != "" && !strings.HasPrefix(dir, e.opts.Directory) {
		dir = filepath.Join(e.opts.Directory, dir)
	}

	out := make([]TestResult, len(tests))

	catcher := &erc.Collector{}
	for idx, t := range tests {
		startAt := time.Now()
		args := []string{e.opts.Interpreter(), "test", "-v"}
		if t.Count > 0 {
			args = append(args, fmt.Sprintf("-count=%d", t.Count))
		}
		if t.Pattern != "" {
			args = append(args, fmt.Sprintf("-run='%s'", t.Pattern))
		}
		if t.Timeout > 0 {
			args = append(args, fmt.Sprintf("-timeout='%s'", t.Timeout.String()))
		}

		args = append(args, t.Args...)

		err := e.manager.CreateCommand(ctx).
			Directory(dir).
			Environment(e.opts.Environment).
			AddEnv("GOPATH", e.opts.Gopath).
			AddEnv("GOROOT", e.opts.Goroot).
			SetOutputOptions(e.opts.Output).
			Add(args).Run(ctx)

		if err != nil {
			catcher.Add(fmt.Errorf("golang test %q: %w", t, err))
		}

		out[idx] = t.getResult(ctx, err, startAt)
	}

	return out, catcher.Resolve()
}
