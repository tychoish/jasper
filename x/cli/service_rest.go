package cli

import (
	"context"
	"fmt"

	"errors"

	"github.com/evergreen-ci/service"

	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/recovery"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/jasper/x/remote"
	"github.com/urfave/cli"
)

const (
	restHostEnvVar  = "JASPER_REST_HOST"
	restPortEnvVar  = "JASPER_REST_PORT"
	defaultRESTPort = 2487
)

func serviceCommandREST(cmd string, operation serviceOperation) cli.Command {
	return cli.Command{
		Name:  RESTService,
		Usage: fmt.Sprintf("%s a REST service", cmd),
		Flags: append(serviceFlags(),
			cli.StringFlag{
				Name:   hostFlagName,
				EnvVar: restHostEnvVar,
				Usage:  "the host running the REST service",
				Value:  defaultLocalHostName,
			},
			cli.IntFlag{
				Name:   portFlagName,
				EnvVar: restPortEnvVar,
				Usage:  "the port running the REST service",
				Value:  defaultRESTPort,
			},
		),
		Before: mergeBeforeFuncs(
			validatePort(portFlagName),
			validateLogLevel(logLevelFlagName),
			validateLimits(limitNumFilesFlagName, limitNumProcsFlagName, limitLockedMemoryFlagName, limitVirtualMemoryFlagName),
		),
		Action: func(c *cli.Context) error {
			manager := jasper.NewManager(jasper.ManagerOptions{Synchronized: true})

			daemon := newRESTDaemon(c.String(hostFlagName), c.Int(portFlagName), manager, makeLogger(c))

			config := serviceConfig(RESTService, c, buildRunCommand(c, RESTService))

			if err := operation(daemon, config); !c.Bool(quietFlagName) {
				return err
			}
			return nil
		},
	}
}

type restDaemon struct {
	Host    string
	Port    int
	Manager jasper.Manager
	Logger  *options.LoggerConfig

	exit chan struct{}
}

func newRESTDaemon(host string, port int, manager jasper.Manager, logger *options.LoggerConfig) *restDaemon {
	return &restDaemon{
		Host:    host,
		Port:    port,
		Manager: manager,
		Logger:  logger,
	}
}

func (d *restDaemon) Start(s service.Service) error {
	if d.Logger != nil {
		if err := setupLogger(d.Logger); err != nil {
			return fmt.Errorf("failed to set up logging: %w", err)
		}
	}

	d.exit = make(chan struct{})
	if d.Manager == nil {
		d.Manager = jasper.NewManager(jasper.ManagerOptions{Synchronized: true})
	}

	ctx, cancel := context.WithCancel(context.Background())
	go handleDaemonSignals(ctx, cancel, d.exit)

	go func(ctx context.Context, d *restDaemon) {
		defer recovery.LogStackTraceAndContinue("rest service")
		grip.Error(message.WrapError(d.run(ctx), "error running REST service"))
	}(ctx, d)

	return nil
}

func (d *restDaemon) Stop(s service.Service) error {
	close(d.exit)
	return nil
}

func (d *restDaemon) run(ctx context.Context) error {
	if err := runServices(ctx, d.newService); err != nil {
		return fmt.Errorf("error running REST service: %w", err)
	}
	return nil
}

func (d *restDaemon) newService(ctx context.Context) (util.CloseFunc, error) {
	if d.Manager == nil {
		return nil, errors.New("manager is not set on REST service")
	}
	grip.Infof("starting REST service at '%s:%d'", d.Host, d.Port)
	return newRESTService(ctx, d.Host, d.Port, d.Manager)
}

// newRESTService creates a REST service around the manager serving requests on
// the host and port.
func newRESTService(ctx context.Context, host string, port int, manager jasper.Manager) (util.CloseFunc, error) {
	service := remote.NewRestService(manager)
	app := service.App(ctx)
	app.SetPrefix("jasper")
	if err := app.SetHost(host); err != nil {
		return nil, fmt.Errorf("error setting REST host: %w", err)
	}
	if err := app.SetPort(port); err != nil {
		return nil, fmt.Errorf("error setting REST port: %w", err)
	}

	go func() {
		defer recovery.LogStackTraceAndContinue("rest service")
		grip.Warning(message.WrapError(app.Run(ctx), "error running REST app"))
	}()

	return func() error { return nil }, nil
}
