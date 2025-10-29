package ssh

import (
	"context"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper/executor"
	"github.com/tychoish/jasper/options"
	cryptossh "golang.org/x/crypto/ssh"
)

// ssh runs processes on a remote machine via SSH.
type libssh struct {
	session *cryptossh.Session
	client  *cryptossh.Client
	args    []string
	dir     string
	env     []string
	exited  bool
	exitErr error
	ctx     context.Context
}

func ExecutorResolverLibrary(ctx context.Context, opts *options.Create) options.ResolveExecutor {
	return func(ctx context.Context, args []string) (executor.Executor, error) {
		if opts.Remote == nil {
			return nil, executor.ErrNotConfigured
		}

		opts.Remote.UseSSHLibrary = true
		client, session, err := resolveClient(opts.Remote)
		if err != nil {
			return nil, fmt.Errorf("could not resolve SSH client and session: %w", err)
		}

		return NewSSH(ctx, client, session, args), nil
	}
}

// NewSSH returns an Executor that creates processes over SSH. Callers are
// expected to clean up resources by explicitly calling Close.
func NewSSH(ctx context.Context, client *cryptossh.Client, session *cryptossh.Session, args []string) executor.Executor {
	return &libssh{
		ctx:     ctx,
		session: session,
		client:  client,
		args:    args,
	}
}

// Args returns the arguments to the process.
func (e *libssh) Args() []string {
	return e.args
}

// SetEnv sets the process environment.
func (e *libssh) SetEnv(env []string) {
	e.env = env
}

// Env returns the process environment.
func (e *libssh) Env() []string {
	return e.env
}

// SetDir sets the process working directory.
func (e *libssh) SetDir(dir string) {
	e.dir = dir
}

// Dir returns the process working directory.
func (e *libssh) Dir() string {
	return e.dir
}

// SetStdin sets the process standard input.
func (e *libssh) SetStdin(stdin io.Reader) {
	e.session.Stdin = stdin
}

// SetStdout sets the process standard output.
func (e *libssh) SetStdout(stdout io.Writer) {
	e.session.Stdout = stdout
}

// Stdout returns the standard output of the process.
func (e *libssh) Stdout() io.Writer { return e.session.Stdout }

// SetStderr sets the process standard error.
func (e *libssh) SetStderr(stderr io.Writer) {
	e.session.Stderr = stderr
}

// Stderr returns the standard error of the process.
func (e *libssh) Stderr() io.Writer { return e.session.Stderr }

// Start begins running the process.
func (e *libssh) Start() error {
	args := []string{}
	for _, entry := range e.env {
		args = append(args, fmt.Sprintf("export %s", entry))
	}
	if e.dir != "" {
		args = append(args, fmt.Sprintf("cd %s", e.dir))
	}
	args = append(args, strings.Join(e.args, " "))
	return e.session.Start(strings.Join(args, "\n"))
}

// Wait returns the result of waiting for the remote process to finish.
func (e *libssh) Wait() error {
	catcher := &erc.Collector{}
	e.exitErr = e.session.Wait()
	catcher.Push(e.exitErr)
	e.exited = true
	return catcher.Resolve()
}

// Signal sends a signal to the remote process.
func (e *libssh) Signal(sig syscall.Signal) error {
	return e.session.Signal(syscallToSSHSignal(sig))
}

// PID is not implemented since there is no simple way to get the remote
// process's PID.
func (e *libssh) PID() int {
	return -1
}

// ExitCode returns the exit code of the process, or -1 if the process is not
// finished.
func (e *libssh) ExitCode() int {
	if !e.exited {
		return -1
	}
	if e.exitErr == nil {
		return 0
	}
	sshExitErr, ok := e.exitErr.(*cryptossh.ExitError)
	if !ok {
		return -1
	}
	return sshExitErr.Waitmsg.ExitStatus()
}

// Success returns whether or not the process ran successfully.
func (e *libssh) Success() bool {
	if !e.exited {
		return false
	}
	return e.exitErr == nil
}

// SignalInfo returns information about signals the process has received.
func (e *libssh) SignalInfo() (sig syscall.Signal, signaled bool) {
	if e.exitErr == nil {
		return syscall.Signal(-1), false
	}
	sshExitErr, ok := e.exitErr.(*cryptossh.ExitError)
	if !ok {
		return syscall.Signal(-1), false
	}
	sshSig := cryptossh.Signal(sshExitErr.Waitmsg.Signal())
	return sshToSyscallSignal(sshSig), sshSig != ""
}

// Close closes the SSH connection resources.
func (e *libssh) Close() error {
	catcher := &erc.Collector{}
	if err := e.session.Close(); err != nil && err != io.EOF {
		catcher.Push(fmt.Errorf("error closing SSH session: %w", err))
	}
	if err := e.client.Close(); err != nil && err != io.EOF {
		catcher.Push(fmt.Errorf("error closing SSH client: %w", err))
	}
	return catcher.Resolve()
}

// syscallToSSHSignal converts a syscall.Signal to its equivalent
// cryptossh.Signal.
func syscallToSSHSignal(sig syscall.Signal) cryptossh.Signal {
	switch sig {
	case syscall.SIGABRT:
		return cryptossh.SIGABRT
	case syscall.SIGALRM:
		return cryptossh.SIGALRM
	case syscall.SIGFPE:
		return cryptossh.SIGFPE
	case syscall.SIGHUP:
		return cryptossh.SIGHUP
	case syscall.SIGILL:
		return cryptossh.SIGILL
	case syscall.SIGINT:
		return cryptossh.SIGINT
	case syscall.SIGKILL:
		return cryptossh.SIGKILL
	case syscall.SIGPIPE:
		return cryptossh.SIGPIPE
	case syscall.SIGQUIT:
		return cryptossh.SIGQUIT
	case syscall.SIGSEGV:
		return cryptossh.SIGSEGV
	case syscall.SIGTERM:
		return cryptossh.SIGTERM
	}
	return cryptossh.Signal("")
}

// sshToSyscallSignal converts a cryptossh.Signal to its equivalent
// syscall.Signal.
func sshToSyscallSignal(sig cryptossh.Signal) syscall.Signal {
	switch sig {
	case cryptossh.SIGABRT:
		return syscall.SIGABRT
	case cryptossh.SIGALRM:
		return syscall.SIGALRM
	case cryptossh.SIGFPE:
		return syscall.SIGFPE
	case cryptossh.SIGHUP:
		return syscall.SIGHUP
	case cryptossh.SIGILL:
		return syscall.SIGILL
	case cryptossh.SIGINT:
		return syscall.SIGINT
	case cryptossh.SIGKILL:
		return syscall.SIGKILL
	case cryptossh.SIGPIPE:
		return syscall.SIGPIPE
	case cryptossh.SIGQUIT:
		return syscall.SIGQUIT
	case cryptossh.SIGSEGV:
		return syscall.SIGSEGV
	case cryptossh.SIGTERM:
		return syscall.SIGTERM
	}
	return syscall.Signal(-1)
}
