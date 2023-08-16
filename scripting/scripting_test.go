package scripting

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
)

func isInPath(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

func evgTaskContains(subs string) bool {
	return strings.Contains(os.Getenv("EVR_TASK_ID"), subs)
}

func makeScriptingEnv(t *testing.T, mgr jasper.Manager, opts options.ScriptingHarness) Harness {
	se, err := NewHarness(mgr, opts)
	assert.NotError(t, err)
	return se
}

func TestScriptingHarness(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := jasper.NewManager(jasper.ManagerOptionSet(jasper.ManagerOptions{Synchronized: true}))
	defer manager.Close(ctx)

	tmpdir, err := os.MkdirTemp(testutil.BuildDirectory(), "scripting_tests")
	assert.NotError(t, err)
	defer func() {
		check.NotError(t, os.RemoveAll(tmpdir))
	}()

	type seTest struct {
		Name string
		Case func(*testing.T, options.ScriptingHarness)
	}

	for _, env := range []struct {
		Name           string
		Supported      bool
		DefaultOptions options.ScriptingHarness
		Tests          []seTest
	}{
		{
			Name:      "Roswell",
			Supported: isInPath("ros"),
			DefaultOptions: &options.ScriptingRoswell{
				Path:   filepath.Join(tmpdir, "roswell"),
				Lisp:   "sbcl-bin",
				Output: options.Output{},
			},
			Tests: []seTest{
				{
					Name: "Options",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						assert.Equal(t, "ros", opts.Interpreter())
						assert.NotZero(t, opts.ID())
					},
				},
				{
					Name: "HelloWorldScript",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.NotError(t, se.RunScript(ctx, `(defun main () (print "hello world"))`))
					},
				},
				{
					Name: "RunHelloWorld",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.NotError(t, se.Run(ctx, []string{`(print "hello world")`}))
					},
				},
				{
					Name: "ScriptExitError",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.Error(t, se.RunScript(ctx, `(sb-ext:exit :code 42)`))
					},
				},
			},
		},
		{
			Name:      "Python3",
			Supported: isInPath("python3") && !evgTaskContains("ubuntu"),
			DefaultOptions: &options.ScriptingPython{
				VirtualEnvPath:    filepath.Join(tmpdir, "python3"),
				LegacyPython:      false,
				InterpreterBinary: "python3",
				Output:            options.Output{},
			},
			Tests: []seTest{
				{
					Name: "Options",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						assert.True(t, strings.HasSuffix(opts.Interpreter(), "python"))
						assert.NotZero(t, opts.ID())
					},
				},
				{
					Name: "HelloWorldScript",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.NotError(t, se.RunScript(ctx, `print("hello world")`))
					},
				},
				{
					Name: "RunHelloWorld",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.NotError(t, se.Run(ctx, []string{"-c", `print("hello world")`}))
					},
				},
				{
					Name: "ScriptExitError",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.Error(t, se.RunScript(ctx, `exit(42)`))
					},
				},
			},
		},
		{
			Name:      "Python2",
			Supported: isInPath("python") && !evgTaskContains("windows"),
			DefaultOptions: &options.ScriptingPython{
				VirtualEnvPath:    filepath.Join(tmpdir, "python2"),
				LegacyPython:      true,
				InterpreterBinary: "python",
				Packages:          []string{"wheel"},
				Output:            options.Output{},
			},
			Tests: []seTest{
				{
					Name: "Options",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						assert.True(t, strings.HasSuffix(opts.Interpreter(), "python"))
						assert.NotZero(t, opts.ID())
					},
				},
				{
					Name: "HelloWorldScript",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.NotError(t, se.RunScript(ctx, `print("hello world")`))
					},
				},
				{
					Name: "RunHelloWorld",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.NotError(t, se.Run(ctx, []string{"-c", `print("hello world")`}))
					},
				},
				{
					Name: "ScriptExitError",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.Error(t, se.RunScript(ctx, `exit(42)`))
					},
				},
			},
		},
		{
			Name:      "Golang",
			Supported: isInPath("go"),
			DefaultOptions: &options.ScriptingGolang{
				Gopath: filepath.Join(tmpdir, "gopath"),
				Goroot: runtime.GOROOT(),
				Packages: []string{
					"github.com/tychoish/fun",
				},
				Output: options.Output{},
			},
			Tests: []seTest{
				{
					Name: "Options",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						assert.True(t, strings.HasSuffix(opts.Interpreter(), "go"))
						assert.NotZero(t, opts.ID())
					},
				},
				{
					Name: "HelloWorldScript",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.NotError(t, se.RunScript(ctx, `package main; import "fmt"; func main() { fmt.Println("Hello World")}`))
					},
				},
				{
					Name: "ScriptExitError",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						assert.Error(t, se.RunScript(ctx, `package main; import "os"; func main() { os.Exit(42) }`))
					},
				},
				{
					Name: "Dependencies",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						se := makeScriptingEnv(t, manager, opts)
						tmpFile := filepath.Join(tmpdir, "fake_script.go")
						assert.NotError(t, os.WriteFile(tmpFile, []byte(`package main; import ("fmt";"errors"; ); func main() { fmt.Println(errors.New("error")) }`), 0755))
						defer func() {
							check.NotError(t, os.Remove(tmpFile))
						}()
						err = se.Run(ctx, []string{tmpFile})
						assert.NotError(t, err)
					},
				},
				{
					Name: "RunFile",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						if runtime.GOOS == "windows" {
							t.Skip("windows paths")
						}
						se := makeScriptingEnv(t, manager, opts)
						tmpFile := filepath.Join(tmpdir, "fake_script.go")
						assert.NotError(t, os.WriteFile(tmpFile, []byte(`package main; import "os"; func main() { os.Exit(0) }`), 0755))
						defer func() {
							check.NotError(t, os.Remove(tmpFile))
						}()
						err = se.Run(ctx, []string{tmpFile})
						assert.NotError(t, err)
					},
				},
				{
					Name: "Build",
					Case: func(t *testing.T, opts options.ScriptingHarness) {
						if runtime.GOOS == "windows" {
							t.Skip("windows paths")
						}
						se := makeScriptingEnv(t, manager, opts)
						tmpFile := filepath.Join(tmpdir, "fake_script.go")
						assert.NotError(t, os.WriteFile(tmpFile, []byte(`package main; import "os"; func main() { os.Exit(0) }`), 0755))
						defer func() {
							check.NotError(t, os.Remove(tmpFile))
						}()
						_, err := se.Build(ctx, testutil.BuildDirectory(), []string{
							"-o", filepath.Join(tmpdir, "fake_script"),
							tmpFile,
						})
						assert.NotError(t, err)
						_, err = os.Stat(filepath.Join(tmpFile))
						assert.NotError(t, err)
					},
				},
			},
		},
	} {
		t.Run(env.Name, func(t *testing.T) {
			if !env.Supported {
				t.Skipf("%s is not supported in the current system", env.Name)
				return
			}
			assert.NotError(t, env.DefaultOptions.Validate())
			t.Run("Config", func(t *testing.T) {
				start := time.Now()
				se := makeScriptingEnv(t, manager, env.DefaultOptions)
				assert.NotError(t, se.Setup(ctx))
				dur := time.Since(start)
				assert.True(t, se != nil)

				t.Run("ID", func(t *testing.T) {
					assert.Equal(t, env.DefaultOptions.ID(), se.ID())
					check.Equal(t, len(se.ID()), 40)
				})
				t.Run("Caching", func(t *testing.T) {
					start := time.Now()
					assert.NotError(t, se.Setup(ctx))

					check.True(t, time.Since(start) < dur)
				})
			})
			for _, test := range env.Tests {
				t.Run(test.Name, func(t *testing.T) {
					test.Case(t, env.DefaultOptions)
				})
			}
			t.Run("Testing", func(t *testing.T) {
				se := makeScriptingEnv(t, manager, env.DefaultOptions)
				res, err := se.Test(ctx, tmpdir)
				assert.NotError(t, err)
				assert.Equal(t, len(res), 0)
			})
			t.Run("Cleanup", func(t *testing.T) {
				se := makeScriptingEnv(t, manager, env.DefaultOptions)
				assert.NotError(t, se.Cleanup(ctx))
			})

		})
	}
}
