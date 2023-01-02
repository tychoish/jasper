package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/tychoish/emt"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
)

type docker struct {
	execOpts types.ExecConfig
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer

	image    string
	platform string

	client *client.Client
	ctx    context.Context

	containerID    string
	containerMutex sync.RWMutex

	status      Status
	statusMutex sync.RWMutex

	pid      int
	exitCode int
	exitErr  error
	signal   syscall.Signal
}

// NewDocker returns an Executor that creates a process within a Docker
// container. Callers are expected to clean up resources by calling Close.
func NewDocker(ctx context.Context, client *client.Client, platform, image string, args []string) Executor {
	return &docker{
		ctx: ctx,
		execOpts: types.ExecConfig{
			Cmd: args,
		},
		platform: platform,
		image:    image,
		client:   client,
		status:   Unstarted,
		pid:      -1,
		exitCode: -1,
		signal:   syscall.Signal(-1),
	}
}

func (e *docker) Args() []string {
	return e.execOpts.Cmd
}

func (e *docker) SetEnv(env []string) {
	e.execOpts.Env = env
}

func (e *docker) Env() []string {
	return e.execOpts.Env
}

func (e *docker) SetDir(dir string) {
	e.execOpts.WorkingDir = dir
}

func (e *docker) Dir() string {
	return e.execOpts.WorkingDir
}

func (e *docker) SetStdin(stdin io.Reader) {
	e.stdin = stdin
	e.execOpts.AttachStdin = stdin != nil
}

func (e *docker) SetStdout(stdout io.Writer) {
	e.stdout = stdout
	e.execOpts.AttachStdout = stdout != nil
}

func (e *docker) Stdout() io.Writer {
	return e.stdout
}

func (e *docker) SetStderr(stderr io.Writer) {
	e.stderr = stderr
	e.execOpts.AttachStderr = stderr != nil
}

func (e *docker) Stderr() io.Writer {
	return e.stderr
}

func (e *docker) Start() error {
	if e.getStatus().After(Unstarted) {
		return errors.New("cannot start a process that has already started, exited, or closed")
	}

	if err := e.setupContainer(); err != nil {
		return fmt.Errorf("could not set up container for process: %w", err)
	}

	if err := e.startContainer(); err != nil {
		return fmt.Errorf("could not start process within container: %w", err)
	}

	e.setStatus(Running)

	return nil
}

// setupContainer creates a container for the process without starting it.
func (e *docker) setupContainer() error {
	containerName := uuid.New().String()
	createResp, err := e.client.ContainerCreate(e.ctx,
		&container.Config{
			Image:        e.image,
			Cmd:          e.execOpts.Cmd,
			Env:          e.execOpts.Env,
			WorkingDir:   e.execOpts.WorkingDir,
			AttachStdin:  e.execOpts.AttachStdin,
			StdinOnce:    e.execOpts.AttachStdin,
			OpenStdin:    e.execOpts.AttachStdin,
			AttachStdout: e.execOpts.AttachStdout,
			AttachStderr: e.execOpts.AttachStderr,
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		&v1.Platform{},
		containerName)
	if err != nil {
		return fmt.Errorf("problem creating container for process: %w", err)
	}
	grip.WarningWhen(len(createResp.Warnings) != 0, message.Fields{
		"message":  "warnings during container creation for process",
		"warnings": createResp.Warnings,
	})

	e.setContainerID(createResp.ID)

	return nil
}

// startContainer attaches any I/O stream to the process and starts the
// container.
func (e *docker) startContainer() error {
	if err := e.setupIOStream(); err != nil {
		return e.withRemoveContainer(fmt.Errorf("problem setting up I/O streaming to process in container: %w", err))
	}

	if err := e.client.ContainerStart(e.ctx, e.getContainerID(), types.ContainerStartOptions{}); err != nil {
		return e.withRemoveContainer(fmt.Errorf("problem starting container for process: %w", err))
	}

	return nil
}

// setupIOStream sets up the process to read standard input and write to
// standard output and standard error. This is a no-op if there are no
// configured inputs or outputs.
func (e *docker) setupIOStream() error {
	if e.stdin == nil && e.stdout == nil && e.stderr == nil {
		return nil
	}

	stream, err := e.client.ContainerAttach(e.ctx, e.getContainerID(), types.ContainerAttachOptions{
		Stream: true,
		Stdin:  e.execOpts.AttachStdin,
		Stdout: e.execOpts.AttachStdout,
		Stderr: e.execOpts.AttachStderr,
	})
	if err != nil {
		return fmt.Errorf("could not set attach I/O to process in container: %w", err)
	}

	go e.runIOStream(stream)

	return nil
}

// runIOStream starts the goroutines to handle standard I/O and waits until the
// stream is done.
func (e *docker) runIOStream(stream types.HijackedResponse) {
	defer stream.Close()
	var wg sync.WaitGroup

	if e.stdin != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := io.Copy(stream.Conn, e.stdin)
			grip.Error(fmt.Errorf("problem streaming input to process: %w", err))
			if err := stream.CloseWrite(); err != nil {
				grip.Error(message.WrapError(err, "problem closing process input stream"))
			}
		}()
	}

	if e.stdout != nil || e.stderr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stdout := e.stdout
			stderr := e.stderr
			if stdout == nil {
				stdout = io.Discard
			}
			if stderr == nil {
				stderr = io.Discard
			}
			if _, err := stdcopy.StdCopy(stdout, stderr, stream.Reader); err != nil {
				grip.Error(fmt.Errorf("problem streaming output from process: %w", err))
			}
		}()
	}

	wg.Wait()
}

// withRemoveContainer returns the error as well as any error from cleaning up
// the container.
func (e *docker) withRemoveContainer(err error) error {
	catcher := emt.NewBasicCatcher()
	catcher.Add(err)
	catcher.Add(e.removeContainer())
	return catcher.Resolve()
}

// removeContainer cleans up the container running this process.
func (e *docker) removeContainer() error {
	containerID := e.getContainerID()
	if containerID == "" {
		return nil
	}

	// We must ensure the container is cleaned up, so do not reuse the
	// Executor's context, which may already be done.
	rmCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.client.ContainerRemove(rmCtx, e.containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("problem cleaning up container for process: %w", err)
	}

	e.setContainerID("")

	return nil
}

func (e *docker) Wait() error {
	if e.getStatus().Before(Running) {
		return errors.New("cannot wait on unstarted process")
	}
	if e.getStatus().After(Running) {
		return e.exitErr
	}

	containerID := e.getContainerID()

	waitDone, errs := e.client.ContainerWait(e.ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errs:
		return fmt.Errorf("error waiting for container to finish running: %w", err)
	case <-e.ctx.Done():
		return e.ctx.Err()
	case <-waitDone:
		state, err := e.getProcessState()
		if err != nil {
			return fmt.Errorf("could not get container state after waiting for completion: %w", err)
		}
		if len(state.Error) != 0 {
			e.exitErr = errors.New(state.Error)
		}
		// In order to maintain the same semantics as exec.Command, we have to
		// return an error for non-zero exit codes.
		if (state.ExitCode) != 0 {
			e.exitErr = fmt.Errorf("exit status %d", state.ExitCode)
		}
	}

	e.setStatus(Exited)

	return e.exitErr
}

func (e *docker) Signal(sig syscall.Signal) error {
	if e.getStatus() != Running {
		return errors.New("cannot signal a non-running process")
	}

	dsig, err := syscallToDockerSignal(sig, e.platform)
	if err != nil {
		return fmt.Errorf("could not get Docker equivalent of signal '%d': %w", sig, err)
	}
	if err := e.client.ContainerKill(e.ctx, e.getContainerID(), dsig); err != nil {
		return fmt.Errorf("could not signal process within container: %w", err)
	}

	e.signal = sig

	return nil
}

// PID returns the PID of the process in the container, or -1 if the PID cannot
// be retrieved.
func (e *docker) PID() int {
	if e.pid > -1 || !e.getStatus().BetweenInclusive(Running, Exited) {
		return e.pid
	}

	state, err := e.getProcessState()
	if err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message":   "could not get container PID",
			"op":        "pid",
			"container": e.getContainerID(),
			"executor":  "docker",
		}))
	}

	e.pid = state.Pid

	return e.pid
}

// ExitCode returns the exit code of the process in the container, or -1 if the
// exit code cannot be retrieved.
func (e *docker) ExitCode() int {
	if e.exitCode > -1 || !e.getStatus().BetweenInclusive(Running, Exited) {
		return e.exitCode
	}

	state, err := e.getProcessState()
	if err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message":   "could not get container exit code",
			"container": e.getContainerID(),
			"executor":  "docker",
		}))
		return e.exitCode
	}

	e.exitCode = state.ExitCode

	return e.exitCode
}

func (e *docker) Success() bool {
	if e.getStatus().Before(Exited) {
		return false
	}
	return e.exitErr == nil
}

// SignalInfo returns information about signals that were sent to the process in
// the container. This will only return information about received signals if
// Signal is called.
func (e *docker) SignalInfo() (sig syscall.Signal, signaled bool) {
	return e.signal, e.signal != -1
}

// Close cleans up the container associated with this process executor and
// closes the connection to the Docker daemon.
func (e *docker) Close() error {
	catcher := emt.NewBasicCatcher()
	catcher.Add(e.removeContainer())
	catcher.Add(e.client.Close())
	e.setStatus(Closed)
	return catcher.Resolve()
}

// getProcessState returns information about the state of the process that ran
// inside the container.
func (e *docker) getProcessState() (*types.ContainerState, error) {
	resp, err := e.client.ContainerInspect(e.ctx, e.getContainerID())
	if err != nil {
		return nil, fmt.Errorf("could not inspect container: %w", err)
	}
	if resp.ContainerJSONBase == nil || resp.ContainerJSONBase.State == nil {
		return nil, fmt.Errorf("introspection of container's process is missing state information: %w", err)
	}
	return resp.ContainerJSONBase.State, nil
}

func (e *docker) getContainerID() string {
	e.containerMutex.RLock()
	defer e.containerMutex.RUnlock()
	return e.containerID
}

func (e *docker) setContainerID(id string) {
	e.containerMutex.Lock()
	defer e.containerMutex.Unlock()
	e.containerID = id
}

func (e *docker) getStatus() Status {
	e.statusMutex.RLock()
	defer e.statusMutex.RUnlock()
	return e.status
}

func (e *docker) setStatus(status Status) {
	e.statusMutex.Lock()
	defer e.statusMutex.Unlock()
	if status < e.status && status != Unknown {
		return
	}
	e.status = status
}
