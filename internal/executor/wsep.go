package executor

import (
	"context"
	"fmt"
	"io"
	"syscall"

	"cdr.dev/wsep"
	"github.com/deciduosity/grip"
	"github.com/pkg/errors"
	"nhooyr.io/websocket"
)

type wsepExec struct {
	ctx      context.Context
	cancel   context.CancelFunc
	client   wsep.Execer
	command  wsep.Command
	proc     wsep.Process
	catcher  grip.Catcher
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	exitCode int
	sigSent  syscall.Signal
}

func NewWsepExecer(ctx context.Context, client wsep.Execer, args []string) Executor {
	pctx, pcancel := context.WithCancel(ctx)
	return &wsepExec{
		ctx:      pctx,
		cancel:   pcancel,
		client:   client,
		catcher:  grip.NewBasicCatcher(),
		exitCode: -1,
		sigSent:  -1,
		command: wsep.Command{
			Args: args,
		},
	}
}

func MakeWsepExecer(ctx context.Context, conn *websocket.Conn, args []string) Executor {
	return NewWsepExecer(ctx, wsep.RemoteExecer(conn), args)
}

func (exec *wsepExec) SetStdin(in io.Reader)   { exec.command.Stdin = true; exec.stdin = in }
func (exec *wsepExec) SetStdout(out io.Writer) { exec.stdout = out }
func (exec *wsepExec) Stdout() io.Writer       { return exec.stdout }
func (exec *wsepExec) SetStderr(e io.Writer)   { exec.stderr = e }
func (exec *wsepExec) Stderr() io.Writer       { return exec.stderr }
func (exec *wsepExec) Args() []string          { return exec.command.Args }
func (exec *wsepExec) SetEnv(e []string)       { exec.command.Env = append(exec.command.Env, e...) }
func (exec *wsepExec) Env() []string           { return exec.command.Env }
func (exec *wsepExec) SetDir(d string)         { exec.command.WorkingDir = d }
func (exec *wsepExec) Dir() string             { return exec.command.WorkingDir }
func (exec *wsepExec) PID() int                { return exec.proc.Pid() }
func (exec *wsepExec) Close() error            { exec.cancel(); return exec.proc.Close() }

func (exec *wsepExec) ExitCode() int                      { return exec.exitCode }
func (exec *wsepExec) Success() bool                      { return exec.exitCode < 1 }
func (exec *wsepExec) SignalInfo() (syscall.Signal, bool) { return exec.sigSent, exec.sigSent >= 0 }

func (exec *wsepExec) Signal(sig syscall.Signal) error {
	if exec.proc == nil {
		return errors.New("cannot signal process before it starts")
	}

	if exec.exitCode >= 0 {
		return errors.New("cannot signal terminated process")
	}

	proc, err := exec.client.Start(exec.ctx, wsep.Command{
		Command: "kill",
		Args:    []string{fmt.Sprint("-", sig), fmt.Sprint(exec.proc.Pid())},
	})
	if err != nil {
		return err
	}

	exec.sigSent = sig

	return proc.Wait()
}

func (exec *wsepExec) Wait() error {
	err := exec.proc.Wait()
	if err == nil {
		exec.exitCode = 0
	}
	if exitErr, ok := err.(wsep.ExitError); ok {
		exec.exitCode = exitErr.Code
	}
	exec.catcher.Add(err)

	// should be non-zero and also an impossible value because
	// exit codes are usually 8-bit
	exec.exitCode = 256

	return exec.catcher.Resolve()
}

func (exec *wsepExec) Start() error {
	if err := exec.startProc(); err != nil {
		return err
	}

	if exec.command.Stdin {
		go func() {
			defer exec.recoveryHandler("standard input")
			stdin := exec.proc.Stdin()
			defer func() { exec.catcher.Add(stdin.Close()) }()
			_, err := io.Copy(stdin, exec.stdin)
			exec.catcher.AddWhen(err != io.EOF, err)
		}()
	}

	go func() {
		defer exec.recoveryHandler("standard output")
		_, err := io.Copy(exec.stdout, exec.proc.Stdout())
		exec.catcher.AddWhen(err != io.EOF, err)
	}()

	go func() {
		defer exec.recoveryHandler("standard error")
		_, err := io.Copy(exec.stdout, exec.proc.Stdout())
		exec.catcher.AddWhen(err != io.EOF, err)
	}()

	return nil
}

func (exec *wsepExec) startProc() error {
	var err error
	exec.proc, err = exec.client.Start(exec.ctx, exec.command)
	if err != nil {
		return err
	}
	return nil
}

func (exec *wsepExec) recoveryHandler(name string) {
	pr := recover()
	if pr == nil {
		return
	}
	exec.catcher.Errorf("handling %s: %v", name, pr)
}
