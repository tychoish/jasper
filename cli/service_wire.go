package cli

import (
	"context"
	"fmt"
	"net"

	"github.com/evergreen-ci/service"
	"github.com/pkg/errors"

	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/remote"
	"github.com/tychoish/jasper/util"
	"github.com/urfave/cli"
)

const (
	wireHostEnvVar  = "JASPER_WIRE_HOST"
	wirePortEnvVar  = "JASPER_WIRE_PORT"
	defaultWirePort = 2488
)

func serviceCommandWire(cmd string, operation serviceOperation) cli.Command {
	return cli.Command{
		Name:  WireService,
		Usage: fmt.Sprintf("%s a MongoDB wire protocol service", cmd),
		Flags: append(serviceFlags(),
			cli.StringFlag{
				Name:   hostFlagName,
				EnvVar: wireHostEnvVar,
				Usage:  "the host running the wire service",
				Value:  defaultLocalHostName,
			},
			cli.IntFlag{
				Name:   portFlagName,
				EnvVar: wirePortEnvVar,
				Usage:  "the port running the wire service",
				Value:  defaultWirePort,
			},
		),
		Before: mergeBeforeFuncs(
			validatePort(portFlagName),
			validateLogLevel(logLevelFlagName),
			validateLimits(limitNumFilesFlagName, limitNumProcsFlagName, limitLockedMemoryFlagName, limitVirtualMemoryFlagName),
		),
		Action: func(c *cli.Context) error {
			manager, err := jasper.NewSynchronizedManager(false)
			if err != nil {
				return fmt.Errorf("error creating wire manager: %w", err)
			}

			daemon := newWireDaemon(c.String(hostFlagName), c.Int(portFlagName), manager, makeLogger(c))

			config := serviceConfig(WireService, c, buildRunCommand(c, WireService))

			if err := operation(daemon, config); !c.Bool(quietFlagName) {
				return err
			}
			return nil
		},
	}
}

type wireDaemon struct {
	Host    string
	Port    int
	Manager jasper.Manager
	Logger  *options.LoggerConfig

	exit chan struct{}
}

func newWireDaemon(host string, port int, manager jasper.Manager, logger *options.LoggerConfig) *wireDaemon {
	return &wireDaemon{
		Host:    host,
		Port:    port,
		Manager: manager,
		Logger:  logger,
	}
}

func (d *wireDaemon) Start(s service.Service) error {
	if d.Logger != nil {
		if err := setupLogger(d.Logger); err != nil {
			return fmt.Errorf("failed to set up logging: %w", err)
		}
	}

	d.exit = make(chan struct{})
	if d.Manager == nil {
		var err error
		if d.Manager, err = jasper.NewSynchronizedManager(false); err != nil {
			return fmt.Errorf("failed to construct wire manager: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go handleDaemonSignals(ctx, cancel, d.exit)

	go func(ctx context.Context, d *wireDaemon) {
		grip.Error(errors.Wrap(d.run(ctx), "error running wire service"))
	}(ctx, d)

	return nil
}

func (d *wireDaemon) Stop(s service.Service) error {
	close(d.exit)
	return nil
}

func (d *wireDaemon) run(ctx context.Context) error {
	return errors.Wrap(runServices(ctx, d.newService), "error running wire service")
}

func (d *wireDaemon) newService(ctx context.Context) (util.CloseFunc, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", d.Host, d.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve wire address: %w", err)
	}

	closeService, err := remote.StartMDBService(ctx, d.Manager, addr)
	if err != nil {
		return nil, fmt.Errorf("error starting wire service: %w", err)
	}
	return closeService, nil
}
