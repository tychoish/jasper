package jasper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/shlex"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/recovery"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper/options"
)

// Command objects allow a quick and lightweight interface for firing off
// ad-hoc processes for smaller tasks. Command immediately supports features
// such as output and error functionality and remote execution. Command methods
// are not thread-safe.
type Command struct {
	opts     options.Command
	procs    []Process
	runFunc  func(options.Command) error
	makeProc ProcessConstructor
}

func (c *Command) sudoCmd() []string {
	sudoCmd := []string{}
	if c.opts.Sudo {
		sudoCmd = append(sudoCmd, "sudo")
		if c.opts.SudoUser != "" {
			sudoCmd = append(sudoCmd, "-u", c.opts.SudoUser)
		}
	}
	return sudoCmd
}

func splitCmdToArgs(cmd string) []string {
	args, err := shlex.Split(cmd)
	if err != nil {
		grip.Error(message.WrapError(err, message.Fields{"input": cmd}))
		return nil
	}
	return args
}

// NewCommand returns a blank Command.
// New blank Commands will use basicProcess as their default Process for
// executing sub-commands unless it is changed via ProcConstructor().
func NewCommand() *Command {
	return &Command{makeProc: NewBasicProcess,
		opts: options.Command{Priority: level.Debug},
	}
}

// ProcConstructor returns a blank Command that will use the process created
// by the given ProcessConstructor.
func (c *Command) ProcConstructor(processConstructor ProcessConstructor) *Command {
	c.makeProc = processConstructor
	return c
}

func (c *Command) GetProcessConstructor() ProcessConstructor { return c.makeProc }

// SetRunFunc sets the function that overrides the default behavior when a
// command is run, allowing the caller to run the command with their own custom
// function given all the given inputs to the command.
func (c *Command) SetRunFunc(f func(options.Command) error) *Command { c.runFunc = f; return c }

// GetProcIDs returns an array of Process IDs associated with the sub-commands
// being run. This method will return a nil slice until processes have actually
// been created by the Command for execution.
func (c *Command) GetProcIDs() []string {
	ids := []string{}
	for _, proc := range c.procs {
		ids = append(ids, proc.ID())
	}
	return ids
}

// ApplyFromOpts uses the options.Create to configure the Command. All existing
// options will be overwritten. Use of this function is discouraged unless all
// desired options are populated in the given opts.
// If Args is set on the options.Create, it will be ignored; the command
// arguments can be added using Add, Append, AppendArgs, or Extend.
func (c *Command) ApplyFromOpts(opts *options.Create) *Command {
	c.opts.Process = *opts
	return c
}

func (c *Command) WithOptions(mod func(*options.Command)) *Command {
	mod(&c.opts)
	return c
}

// SetOutputOptions sets the output options for a command. This overwrites an
// existing output options.
func (c *Command) SetOutputOptions(opts options.Output) *Command {
	c.opts.Process.Output = opts
	return c
}

// String returns a stringified representation.
func (c *Command) String() string {
	var remote string
	if c.opts.Process.Remote == nil {
		remote = "nil"
	} else {
		remote = c.opts.Process.Remote.String()
	}

	return fmt.Sprintf("id='%s', remote='%s', cmd='%s'", c.opts.ID, remote, c.getCmd())
}

// Export returns all of the options.Create that will be used to spawn the
// processes that run all subcommands.
func (c *Command) Export() ([]*options.Create, error) {
	opts, err := c.ExportCreateOptions()
	if err != nil {
		return nil, fmt.Errorf("problem getting process creation options: %w", err)
	}
	return opts, nil
}

func (c *Command) initRemote() {
	if c.opts.Process.Remote == nil {
		c.opts.Process.Remote = &options.Remote{}
	}
}

func (c *Command) initRemoteProxy() {
	if c.opts.Process.Remote == nil {
		c.opts.Process.Remote = &options.Remote{}
	}
	if c.opts.Process.Remote.Proxy == nil {
		c.opts.Process.Remote.Proxy = &options.Proxy{}
	}
}

// Host sets the hostname for connecting to a remote host.
func (c *Command) Host(h string) *Command {
	c.initRemote()
	c.opts.Process.Remote.Host = h
	return c
}

// User sets the username for connecting to a remote host.
func (c *Command) User(u string) *Command {
	c.initRemote()
	c.opts.Process.Remote.User = u
	return c
}

// Port sets the port for connecting to a remote host.
func (c *Command) Port(p int) *Command {
	c.initRemote()
	c.opts.Process.Remote.Port = p
	return c
}

// ExtendRemoteArgs allows you to add arguments, when needed, to the
// Password sets the password in order to authenticate to a remote host.
// underlying ssh command, for remote commands.
func (c *Command) ExtendRemoteArgs(args ...string) *Command {
	c.initRemote()
	c.opts.Process.Remote.Args = append(c.opts.Process.Remote.Args, args...)
	return c
}

// PrivKey sets the private key in order to authenticate to a remote host.
func (c *Command) PrivKey(key string) *Command {
	c.initRemote()
	c.opts.Process.Remote.Key = key
	return c
}

// PrivKeyFile sets the path to the private key file in order to authenticate to
// a remote host.
func (c *Command) PrivKeyFile(path string) *Command {
	c.initRemote()
	c.opts.Process.Remote.KeyFile = path
	return c
}

// PrivKeyPassphrase sets the passphrase for the private key file in order to
// authenticate to a remote host.
func (c *Command) PrivKeyPassphrase(pass string) *Command {
	c.initRemote()
	c.opts.Process.Remote.KeyPassphrase = pass
	return c
}

// Password sets the password in order to authenticate to a remote host.
func (c *Command) Password(p string) *Command {
	c.initRemote()
	c.opts.Process.Remote.Password = p
	return c
}

// ProxyHost sets the proxy hostname for connecting to a proxy host.
func (c *Command) ProxyHost(h string) *Command {
	c.initRemoteProxy()
	c.opts.Process.Remote.Proxy.Host = h
	return c
}

// ProxyUser sets the proxy username for connecting to a proxy host.
func (c *Command) ProxyUser(u string) *Command {
	c.initRemoteProxy()
	c.opts.Process.Remote.Proxy.User = u
	return c
}

// ProxyPort sets the proxy port for connecting to a proxy host.
func (c *Command) ProxyPort(p int) *Command {
	c.initRemoteProxy()
	c.opts.Process.Remote.Proxy.Port = p
	return c
}

// ProxyPrivKey sets the proxy private key in order to authenticate to a remote host.
func (c *Command) ProxyPrivKey(key string) *Command {
	c.initRemoteProxy()
	c.opts.Process.Remote.Proxy.Key = key
	return c
}

// ProxyPrivKeyFile sets the path to the proxy private key file in order to
// authenticate to a proxy host.
func (c *Command) ProxyPrivKeyFile(path string) *Command {
	c.initRemoteProxy()
	c.opts.Process.Remote.Proxy.KeyFile = path
	return c
}

// ProxyPrivKeyPassphrase sets the passphrase for the private key file in order to
// authenticate to a proxy host.
func (c *Command) ProxyPrivKeyPassphrase(pass string) *Command {
	c.initRemoteProxy()
	c.opts.Process.Remote.Proxy.KeyPassphrase = pass
	return c
}

// ProxyPassword sets the password in order to authenticate to a proxy host.
func (c *Command) ProxyPassword(p string) *Command {
	c.initRemoteProxy()
	c.opts.Process.Remote.Proxy.Password = p
	return c
}

// SetRemoteOptions sets the configuration for remote operations. This overrides
// any existing remote configuration.
func (c *Command) SetRemoteOptions(opts *options.Remote) *Command {
	c.opts.Process.Remote = opts
	return c
}

// Directory sets the working directory. If this is a remote command, it sets
// the working directory of the command being run remotely.
func (c *Command) Directory(d string) *Command { c.opts.Process.WorkingDirectory = d; return c }

// Priority sets the logging priority.
func (c *Command) Priority(l level.Priority) *Command { c.opts.Priority = l; return c }

// ID sets the ID of the Command, which is independent of the IDs of the
// subcommands that are executed.
func (c *Command) ID(id string) *Command { c.opts.ID = id; return c }

// SetTags overrides any existing tags for a process with the
// specified list. Tags are used to filter process with the manager.
func (c *Command) SetTags(tags []string) *Command { c.opts.Process.Tags = tags; return c }

// AppendTags adds the specified tags to the existing tag slice. Tags
// are used to filter process with the manager.
func (c *Command) AppendTags(t ...string) *Command {
	c.opts.Process.Tags = append(c.opts.Process.Tags, t...)
	return c
}

// ExtendTags adds all tags in the specified slice to the tags will be
// added to the process after creation. Tags are used to filter
// process with the manager.
func (c *Command) ExtendTags(t []string) *Command {
	c.opts.Process.Tags = append(c.opts.Process.Tags, t...)
	return c
}

// Background allows you to set the command to run in the background
// when you call Run(), the command will begin executing but will not
// complete when run returns.
func (c *Command) Background(runBackground bool) *Command {
	c.opts.RunBackground = runBackground
	return c
}

// ContinueOnError sets a flag for determining if the Command should continue
// executing its sub-commands even if one of them errors.
func (c *Command) ContinueOnError(cont bool) *Command { c.opts.ContinueOnError = cont; return c }

// IgnoreError sets a flag for determining if the Command should return a nil
// error despite errors in its sub-command executions.
func (c *Command) IgnoreError(ignore bool) *Command { c.opts.IgnoreError = ignore; return c }

// SuppressStandardError sets a flag for determining if the Command should
// discard all standard error content.
func (c *Command) SuppressStandardError(v bool) *Command {
	c.opts.Process.Output.SuppressError = v
	return c
}

// SetLoggers sets the logging output on this command to the specified
// slice. This removes any loggers previously configured.
func (c *Command) SetLoggers(l []*options.LoggerConfig) *Command {
	c.opts.Process.Output.Loggers = l
	return c
}

// AppendLoggers adds one or more loggers to the existing configured
// loggers in the command.
func (c *Command) AppendLoggers(l ...*options.LoggerConfig) *Command {
	c.opts.Process.Output.Loggers = append(c.opts.Process.Output.Loggers, l...)
	return c
}

// ExtendLoggers takes the existing slice of loggers and adds that to any
// existing configuration.
func (c *Command) ExtendLoggers(l []*options.LoggerConfig) *Command {
	c.opts.Process.Output.Loggers = append(c.opts.Process.Output.Loggers, l...)
	return c
}

// SuppressStandardOutput sets a flag for determining if the Command should
// discard all standard output content.
func (c *Command) SuppressStandardOutput(v bool) *Command {
	c.opts.Process.Output.SuppressOutput = v
	return c
}

// RedirectOutputToError sets a flag for determining if the Command should send
// all standard output content to standard error.
func (c *Command) RedirectOutputToError(v bool) *Command {
	c.opts.Process.Output.SendOutputToError = v
	return c
}

// RedirectErrorToOutput sets a flag for determining if the Command should send
// all standard error content to standard output.
func (c *Command) RedirectErrorToOutput(v bool) *Command {
	c.opts.Process.Output.SendErrorToOutput = v
	return c
}

// Environment replaces the current environment map with the given environment
// map. If this is a remote command, it sets the environment of the command
// being run remotely.
func (c *Command) Environment(e map[string]string) *Command {
	c.opts.Process.Environment = e
	return c
}

// AddEnv adds a key value pair of environment variable to value into the
// Command's environment variable map. If this is a remote command, it sets the
// environment of the command being run remotely.
func (c *Command) AddEnv(k, v string) *Command {
	c.setupEnv()
	c.opts.Process.Environment[k] = v
	return c
}

// Add adds on a sub-command.
func (c *Command) Add(args []string) *Command {
	c.opts.Commands = append(c.opts.Commands, args)
	return c
}

// AddWhen adds a command only when the conditional is true.
func (c *Command) AddWhen(cond bool, args []string) *Command {
	if !cond {
		return c
	}
	return c.Add(args)
}

// Sudo runs each command with superuser privileges with the default target
// user. This will cause the commands to fail if the commands are executed in
// Windows. If this is a remote command, the command being run remotely uses
// superuser privileges.
func (c *Command) Sudo(sudo bool) *Command { c.opts.Sudo = sudo; return c }

// SudoAs runs each command with sudo but allows each command to be run as a
// user other than the default target user (usually root). This will cause the
// commands to fail if the commands are executed in Windows. If this is a remote
// command, the command being run remotely uses superuser privileges.
func (c *Command) SudoAs(user string) *Command {
	c.opts.Sudo = true
	c.opts.SudoUser = user
	return c
}

// Extend adds on multiple sub-commands.
func (c *Command) Extend(cmds [][]string) *Command {
	c.opts.Commands = append(c.opts.Commands, cmds...)
	return c
}

// ExtendWhen adds on multiple sub-commands, only when the conditional
// is true.
func (c *Command) ExtendWhen(cond bool, cmds [][]string) *Command {
	if !cond {
		return c
	}
	return c.Extend(cmds)
}

// Append takes a series of strings and splits them into sub-commands and adds
// them to the Command.
func (c *Command) Append(cmds ...string) *Command {
	for _, cmd := range cmds {
		c.opts.Commands = append(c.opts.Commands, splitCmdToArgs(cmd))
	}
	return c
}

// AppendWhen adds a sequence of subcommands to the Command only when
// the conditional is true.
func (c *Command) AppendWhen(cond bool, cmds ...string) *Command {
	if !cond {
		return c
	}

	return c.Append(cmds...)
}

// ShellScript adds an operation to the command that runs a shell script, using
// the shell's "-c" option).
func (c *Command) ShellScript(shell, script string) *Command {
	c.opts.Commands = append(c.opts.Commands, []string{shell, "-c", script})
	return c
}

// ShellScriptWhen a shell script option only when the conditional of true.
func (c *Command) ShellScriptWhen(cond bool, shell, script string) *Command {
	if !cond {
		return c
	}

	return c.ShellScript(shell, script)
}

// Bash adds a script using "bash -c", as syntactic sugar for the ShellScript2
// method.
func (c *Command) Bash(script string) *Command { return c.ShellScript("bash", script) }

// BashWhen adds a bash script only when the conditional is true.
func (c *Command) BashWhen(cond bool, script string) *Command {
	if !cond {
		return c

	}

	return c.Bash(script)
}

// Sh adds a script using "sh -c", as syntactic sugar for the ShellScript
// method.
func (c *Command) Sh(script string) *Command { return c.ShellScript("sh", script) }

// ShWhen adds a shell script (e.g. "sh -c") only when the conditional
// is true.
func (c *Command) ShWhen(cond bool, script string) *Command {
	if !cond {
		return c
	}

	return c.Sh(script)
}

// AppendArgs is the variadic equivalent of Add, which adds a command
// in the form of arguments.
func (c *Command) AppendArgs(args ...string) *Command { return c.Add(args) }

// AppendArgsWhen is the variadic equivalent of AddWhen, which adds a command
// in the form of arguments only when the conditional is true.
func (c *Command) AppendArgsWhen(cond bool, args ...string) *Command {
	if !cond {
		return c
	}

	return c.Add(args)
}

// Prerequisite sets a function on the Command such that the Command will only
// execute if the function returns true. The Prerequisite function runs once per
// Command object regardless of how many subcommands are
// present. Prerequsite functions run even if the command's
// RunFunction is set.
func (c *Command) Prerequisite(chk func() bool) *Command { c.opts.Prerequisite = chk; return c }

// PostHook allows you to add a function that runs (locally) after the
// each subcommand in the Command completes. When specified, the
// PostHook can override or annotate any error produced by the command
// execution. The error returned is subject to the IgnoreError
// options. The PostHook is not run when using SetRunFunction.
func (c *Command) PostHook(h options.CommandPostHook) *Command { c.opts.PostHook = h; return c }

// PreHook allows you to add a function that runs (locally) before
// each subcommand executes, and allows you to log or modify the
// creation option. The PreHook is not run when using SetRunFunction.
func (c *Command) PreHook(fn options.CommandPreHook) *Command { c.opts.PreHook = fn; return c }

func (c *Command) setupEnv() {
	if c.opts.Process.Environment == nil {
		c.opts.Process.Environment = map[string]string{}
	}
}

func (c *Command) Worker() fun.Worker { return c.Run }

// Run starts and then waits on the Command's execution.
func (c *Command) Run(ctx context.Context) error {
	if c.opts.Prerequisite != nil && !c.opts.Prerequisite() {
		grip.Debug(message.Fields{
			"op":  "noop after prerequisite returned false",
			"id":  c.opts.ID,
			"cmd": c.String(),
		})
		return nil
	}

	if c.runFunc != nil {
		return c.runFunc(c.opts)
	}

	catcher := &erc.Collector{}

	opts, err := c.ExportCreateOptions()
	if err != nil {
		catcher.Add(err)
		catcher.Add(c.Close())
		return catcher.Resolve()
	}

	for _, opt := range opts {
		if err := ctx.Err(); err != nil {
			catcher.Add(fmt.Errorf("operation canceled: %w", err))
			catcher.Add(c.Close())
			return catcher.Resolve()
		}

		if c.opts.PreHook != nil {
			c.opts.PreHook(&c.opts, opt)
		}

		err := c.exec(ctx, opt)

		if !c.opts.IgnoreError {
			if c.opts.PostHook != nil {
				catcher.Add(c.opts.PostHook(err))
			} else {
				catcher.Add(err)
			}
		}

		if err != nil && !c.opts.ContinueOnError {
			catcher.Add(c.Close())
			return catcher.Resolve()
		}
	}

	catcher.Add(c.Close())
	return catcher.Resolve()
}

// RunParallel is the same as Run(), but will run all sub-commands in parallel.
// Use of this function effectively ignores the ContinueOnError flag.
func (c *Command) RunParallel(ctx context.Context) error {
	// Avoid paying the copy-costs in between command structs by doing the work
	// before executing the commands.
	parallelCmds := make([]Command, len(c.opts.Commands))

	for idx, cmd := range c.opts.Commands {
		splitCmd := *c
		optsCopy := c.opts.Process.Copy()
		splitCmd.opts.Process = *optsCopy
		splitCmd.opts.Commands = [][]string{cmd}
		splitCmd.procs = []Process{}
		parallelCmds[idx] = splitCmd
	}

	type cmdResult struct {
		procs []Process
		err   error
	}
	cmdResults := make(chan cmdResult, len(c.opts.Commands))
	for _, parallelCmd := range parallelCmds {
		go func(innerCmd Command) {
			defer func() {
				err := recovery.HandlePanicWithError(recover(), nil, "parallel command encountered error")
				if err != nil {
					cmdResults <- cmdResult{err: err}
				}
			}()
			err := innerCmd.Run(ctx)
			select {
			case cmdResults <- cmdResult{procs: innerCmd.procs, err: err}:
			case <-ctx.Done():
			}
		}(parallelCmd)
	}

	catcher := &erc.Collector{}
	for i := 0; i < len(c.opts.Commands); i++ {
		select {
		case cmdRes := <-cmdResults:
			if !c.opts.IgnoreError {
				catcher.Add(cmdRes.err)
			}
			c.procs = append(c.procs, cmdRes.procs...)
		case <-ctx.Done():
			c.procs = []Process{}
			catcher.Add(c.Close())
			catcherErr := catcher.Resolve()
			if catcherErr != nil {
				return fmt.Errorf("catcher errors %q: %w", catcherErr.Error(), ctx.Err())
			}
			return ctx.Err()
		}
	}

	catcher.Add(c.Close())
	return catcher.Resolve()
}

// Close closes this command and its resources.
func (c *Command) Close() error {
	return c.opts.Process.Close()
}

// SetInput sets the standard input.
func (c *Command) SetInput(r io.Reader) *Command {
	c.opts.Process.StandardInput = r
	return c
}

// SetInputBytes is the same as SetInput but sets b as the bytes to be read from
// standard input.
func (c *Command) SetInputBytes(b []byte) *Command {
	c.opts.Process.StandardInputBytes = b
	return c
}

// SetErrorSender sets a Sender to be used by this Command for its output to
// stderr.
func (c *Command) SetErrorSender(l level.Priority, s send.Sender) *Command {
	writer := send.MakeWriter(s)
	writer.Set(l)
	c.opts.Process.RegisterCloser(writer.Close)
	c.opts.Process.Output.Error = writer
	return c
}

// SetOutputSender sets a Sender to be used by this Command for its output to
// stdout.
func (c *Command) SetOutputSender(l level.Priority, s send.Sender) *Command {
	writer := send.MakeWriter(s)
	writer.Set(l)
	c.opts.Process.RegisterCloser(writer.Close)
	c.opts.Process.Output.Output = writer
	return c
}

// SetCombinedSender is the combination of SetErrorSender() and
// SetOutputSender().
func (c *Command) SetCombinedSender(l level.Priority, s send.Sender) *Command {
	writer := send.MakeWriter(s)
	writer.Set(l)
	c.opts.Process.RegisterCloser(writer.Close)
	c.opts.Process.Output.Error = writer
	c.opts.Process.Output.Output = writer
	return c
}

// SetErrorWriter sets a Writer to be used by this Command for its output to
// stderr.
func (c *Command) SetErrorWriter(writer io.WriteCloser) *Command {
	c.opts.Process.RegisterCloser(writer.Close)
	c.opts.Process.Output.Error = writer
	return c
}

// SetOutputWriter sets a Writer to be used by this Command for its output to
// stdout.
func (c *Command) SetOutputWriter(writer io.WriteCloser) *Command {
	c.opts.Process.RegisterCloser(writer.Close)
	c.opts.Process.Output.Output = writer
	return c
}

// SetCombinedWriter is the combination of SetErrorWriter() and
// SetOutputWriter().
func (c *Command) SetCombinedWriter(writer io.WriteCloser) *Command {
	c.opts.Process.RegisterCloser(writer.Close)
	c.opts.Process.Output.Error = writer
	c.opts.Process.Output.Output = writer
	return c
}

func (c *Command) getCmd() string {
	env := strings.Join(c.opts.Process.ResolveEnvironment(), " ")
	out := []string{}
	for _, cmd := range c.opts.Commands {
		if c.opts.Sudo {
			cmd = append(c.sudoCmd(), cmd...)
		}
		var formattedCmd string
		if len(env) != 0 {
			formattedCmd = fmt.Sprintf("%s%s ", formattedCmd, env)
		}
		formattedCmd = strings.Join(append([]string{formattedCmd}, cmd...), " ")
		out = append(out, formattedCmd)
	}
	return strings.Join(out, "\n")
}

func (c *Command) getCreateOpt(args []string) (*options.Create, error) {
	opts := c.opts.Process.Copy()

	switch len(args) {
	case 0:
		return nil, errors.New("cannot have empty args")
	case 1:
		if c.opts.Process.Remote == nil && strings.ContainsAny(args[0], " \"'") {
			spl, err := shlex.Split(args[0])
			if err != nil {
				return nil, fmt.Errorf("problem splitting argstring: %w", err)
			}
			return c.getCreateOpt(spl)
		}
		opts.Args = args
	default:
		opts.Args = args
	}

	if c.opts.Sudo {
		opts.Args = append(c.sudoCmd(), opts.Args...)
	}

	return opts, nil
}

func (c *Command) ExportCreateOptions() ([]*options.Create, error) {
	out := make([]*options.Create, 0, len(c.opts.Commands))
	catcher := &erc.Collector{}
	for _, args := range c.opts.Commands {
		cmd, err := c.getCreateOpt(args)
		if err != nil {
			catcher.Add(err)
			continue
		}

		out = append(out, cmd)
	}

	if !catcher.Ok() {
		return nil, catcher.Resolve()
	}

	return out, nil
}

func (c *Command) exec(ctx context.Context, opts *options.Create) error {
	writeOutput := getMsgOutput(opts.Output)
	proc, err := c.makeProc(ctx, opts)
	if err != nil {
		return fmt.Errorf("problem starting command: %w", err)
	}
	c.procs = append(c.procs, proc)
	msg := message.Fields{
		"id":   c.opts.ID,
		"proc": proc.ID(),
	}
	if len(c.opts.Process.Tags) > 0 {
		msg["tags"] = c.opts.Process.Tags
	}

	cstr := strings.Join(opts.Args, " ")
	if len(cstr) > 36 {
		cstr = fmt.Sprintf("(%s)...", strings.Trim(cstr[:36], "- \t"))
	}
	msg["cmd"] = cstr

	if opts.WorkingDirectory != ft.Must(os.Getwd()) {
		msg["path"] = opts.WorkingDirectory
	}

	if !c.opts.RunBackground {
		ec := &erc.Collector{}
		for _, proc := range c.procs {
			_, err = proc.Wait(ctx)
			if err != nil {
				ec.Add(fmt.Errorf("process(%s) group(%s): %w", proc.ID(), c.opts.ID, err))
			}
		}
		err = ec.Resolve()
		if err != nil {
			msg["err"] = err
		}

		grip.Log(c.opts.Priority, writeOutput(msg))
	}
	return err
}

func getMsgOutput(opts options.Output) func(msg message.Fields) message.Fields {
	noOutput := func(msg message.Fields) message.Fields { return msg }

	if opts.Output != nil && opts.Error != nil {
		return noOutput
	}

	logger, err := NewInMemoryLogger(1000)
	if err != nil {
		return func(msg message.Fields) message.Fields {
			msg["log_err"] = fmt.Errorf("could not set up in-memory sender for capturing output: %w", err)
			return msg
		}
	}
	sender, err := logger.Resolve()
	if err != nil {
		return func(msg message.Fields) message.Fields {
			msg["log_err"] = fmt.Errorf("could not set up in-memory sender for capturing output: %w", err)
			return msg
		}
	}

	writer := send.MakeWriter(sender)
	if opts.Output == nil {
		opts.Output = writer
	}
	if opts.Error == nil {
		opts.Error = writer
	}

	return func(msg message.Fields) message.Fields {
		inMemorySender, ok := sender.(*send.InMemorySender)
		if !ok {
			return msg
		}
		logs, err := inMemorySender.GetString()
		if err != nil {
			msg["log_err"] = err
			return msg
		}
		msg["output"] = strings.Join(logs, "\n")
		return msg
	}
}

// Wait returns the exit code and error waiting for the underlying process to
// complete.
// For commands run with RunParallel, Wait only returns a zero exit code if all
// the underlying processes return exit code zero; otherwise, it returns a
// non-zero exit code. Similarly, it will return a non-nil error if any of the
// underlying processes encounter an error while waiting.
func (c *Command) Wait(ctx context.Context) (int, error) {
	if len(c.procs) == 0 {
		return 0, errors.New("cannot call wait on a command if no processes have started yet")
	}

	for _, proc := range c.procs {
		exitCode, err := proc.Wait(ctx)
		if err != nil || exitCode != 0 {
			return exitCode, fmt.Errorf("error waiting on process '%s': %w", proc.ID(), err)
		}
	}

	return 0, nil
}

// BuildCommand builds the Command given the configuration of arguments.
func BuildCommand(id string, pri level.Priority, args []string, dir string, env map[string]string) *Command {
	return NewCommand().ID(id).Priority(pri).Add(args).Directory(dir).Environment(env)
}

// BuildRemoteCommand builds the Command remotely given the configuration of arguments.
func BuildRemoteCommand(id string, pri level.Priority, host string, args []string, dir string, env map[string]string) *Command {
	return NewCommand().ID(id).Priority(pri).Host(host).Add(args).Directory(dir).Environment(env)
}

// BuildCommandGroupContinueOnError runs the group of sub-commands given the
// configuration of arguments, continuing execution despite any errors.
func BuildCommandGroupContinueOnError(id string, pri level.Priority, cmds [][]string, dir string, env map[string]string) *Command {
	return NewCommand().ID(id).Priority(pri).Extend(cmds).Directory(dir).Environment(env).ContinueOnError(true)
}

// BuildRemoteCommandGroupContinueOnError runs the group of sub-commands remotely
// given the configuration of arguments, continuing execution despite any
// errors.
func BuildRemoteCommandGroupContinueOnError(id string, pri level.Priority, host string, cmds [][]string, dir string) *Command {
	return NewCommand().ID(id).Priority(pri).Host(host).Extend(cmds).Directory(dir).ContinueOnError(true)
}

// BuildCommandGroup runs the group of sub-commands given the configuration of
// arguments.
func BuildCommandGroup(id string, pri level.Priority, cmds [][]string, dir string, env map[string]string) *Command {
	return NewCommand().ID(id).Priority(pri).Extend(cmds).Directory(dir).Environment(env)
}

// BuildRemoteCommandGroup runs the group of sub-commands remotely given the
// configuration of arguments.
func BuildRemoteCommandGroup(id string, pri level.Priority, host string, cmds [][]string, dir string) *Command {
	return NewCommand().ID(id).Priority(pri).Host(host).Extend(cmds).Directory(dir)
}
