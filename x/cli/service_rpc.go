package cli

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/evergreen-ci/service"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/recovery"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/jasper/x/remote"
	"github.com/urfave/cli/v2"
)

const (
	rpcHostEnvVar  = "JASPER_RPC_HOST"
	rpcPortEnvVar  = "JASPER_RPC_PORT"
	defaultRPCPort = 2486
)

func serviceCommandRPC(cmd string, operation serviceOperation) *cli.Command {
	return &cli.Command{
		Name:  RPCService,
		Usage: fmt.Sprintf("%s an RPC service", cmd),
		Flags: append(serviceFlags(),
			&cli.StringFlag{
				Name:    hostFlagName,
				EnvVars: []string{rpcHostEnvVar},
				Usage:   "the host running the RPC service",
				Value:   defaultLocalHostName,
			},
			&cli.IntFlag{
				Name:    portFlagName,
				EnvVars: []string{rpcPortEnvVar},
				Usage:   "the port running the RPC service",
				Value:   defaultRPCPort,
			},
			&cli.StringFlag{
				Name:  credsFilePathFlagName,
				Usage: "the path to the file containing the RPC service credentials",
			},
		),
		Before: mergeBeforeFuncs(
			validatePort(portFlagName),
			validateLogLevel(logLevelFlagName),
			validateLimits(limitNumFilesFlagName, limitNumProcsFlagName, limitLockedMemoryFlagName, limitVirtualMemoryFlagName),
		),
		Action: func(c *cli.Context) error {
			manager := jasper.NewManager(jasper.ManagerOptions{Synchronized: true})

			daemon := newRPCDaemon(c.String(hostFlagName), c.Int(portFlagName), manager, c.String(credsFilePathFlagName), makeLogger(c))

			config := serviceConfig(RPCService, c, buildRunCommand(c, RPCService))

			if err := operation(daemon, config); !c.Bool(quietFlagName) {
				return err
			}
			return nil
		},
	}
}

type rpcDaemon struct {
	Host          string
	Port          int
	CredsFilePath string
	Manager       jasper.Manager
	Logger        *options.LoggerConfig

	exit chan struct{}
}

func newRPCDaemon(host string, port int, manager jasper.Manager, credsFilePath string, logger *options.LoggerConfig) *rpcDaemon {
	return &rpcDaemon{
		Host:          host,
		Port:          port,
		CredsFilePath: credsFilePath,
		Manager:       manager,
		Logger:        logger,
	}
}

func (d *rpcDaemon) Start(s service.Service) error {
	if d.Logger != nil {
		if err := setupLogger(d.Logger); err != nil {
			return fmt.Errorf(": %w", err)
		}
	}

	d.exit = make(chan struct{})
	if d.Manager == nil {
		d.Manager = jasper.NewManager(jasper.ManagerOptions{Synchronized: true})
	}

	ctx, cancel := context.WithCancel(context.Background())
	go handleDaemonSignals(ctx, cancel, d.exit)

	go func(ctx context.Context, d *rpcDaemon) {
		defer recovery.LogStackTraceAndContinue("rpc service")
		grip.Error(message.WrapError(d.run(ctx), "error running RPC service"))
	}(ctx, d)

	return nil
}

func (d *rpcDaemon) Stop(s service.Service) error {
	close(d.exit)
	return nil
}

func (d *rpcDaemon) run(ctx context.Context) error {
	if err := runServices(ctx, d.newService); err != nil {
		return fmt.Errorf("error running RPC service: %w", err)
	}
	return nil
}

func (d *rpcDaemon) newService(ctx context.Context) (util.CloseFunc, error) {
	if d.Manager == nil {
		return nil, errors.New("manager is not set on RPC service")
	}

	grip.Infof("starting RPC service at '%s:%d'", d.Host, d.Port)

	return newRPCService(ctx, d.Host, d.Port, d.Manager, d.CredsFilePath)
}

// newRPCService creates an RPC service around the manager serving requests on
// the host and port.
func newRPCService(ctx context.Context, host string, port int, manager jasper.Manager, credsFilePath string) (util.CloseFunc, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve RPC address: %w", err)
	}

	closeService, err := remote.StartRPCServiceWithFile(ctx, manager, addr, credsFilePath)
	if err != nil {
		return nil, fmt.Errorf("error starting RPC service: %w", err)
	}
	return closeService, nil
}
