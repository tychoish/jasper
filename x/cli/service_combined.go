package cli

import (
	"fmt"

	"github.com/evergreen-ci/service"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper"
	"github.com/urfave/cli"
)

const (
	restHostFlagName = "rest_host"
	restPortFlagName = "rest_port"

	rpcHostFlagName          = "rpc_host"
	rpcPortFlagName          = "rpc_port"
	rpcCredsFilePathFlagName = "rpc_creds_path"
)

func serviceCommandCombined(cmd string, operation serviceOperation) cli.Command {
	return cli.Command{
		Name:  CombinedService,
		Usage: fmt.Sprintf("%s a combined service", cmd),
		Flags: append(serviceFlags(),
			cli.StringFlag{
				Name:   restHostFlagName,
				EnvVar: restHostEnvVar,
				Usage:  "the host running the REST service ",
				Value:  defaultLocalHostName,
			},
			cli.IntFlag{
				Name:   restPortFlagName,
				EnvVar: restPortEnvVar,
				Usage:  "the port running the REST service ",
				Value:  defaultRESTPort,
			},
			cli.StringFlag{
				Name:   rpcHostFlagName,
				EnvVar: rpcHostEnvVar,
				Usage:  "the host running the RPC service ",
				Value:  defaultLocalHostName,
			},
			cli.IntFlag{
				Name:   rpcPortFlagName,
				EnvVar: rpcPortEnvVar,
				Usage:  "the port running the RPC service",
				Value:  defaultRPCPort,
			},
			cli.StringFlag{
				Name:  rpcCredsFilePathFlagName,
				Usage: "the path to the RPC service credentials file",
			},
		),
		Before: mergeBeforeFuncs(
			validatePort(restPortFlagName),
			validatePort(rpcPortFlagName),
			validateLogLevel(logLevelFlagName),
			validateLimits(limitNumFilesFlagName, limitNumProcsFlagName, limitLockedMemoryFlagName, limitVirtualMemoryFlagName),
		),
		Action: func(c *cli.Context) error {
			manager := jasper.NewManager(jasper.ManagerOptions{Synchronized: true})

			daemon := newCombinedDaemon(
				newRESTDaemon(c.String(restHostFlagName), c.Int(restPortFlagName), manager, makeLogger(c)),
				newRPCDaemon(c.String(rpcHostFlagName), c.Int(rpcPortFlagName), manager, c.String(rpcCredsFilePathFlagName), makeLogger(c)),
			)

			config := serviceConfig(CombinedService, c, buildRunCommand(c, CombinedService))

			if err := operation(daemon, config); !c.Bool(quietFlagName) {
				return err
			}
			return nil
		},
	}
}

type combinedDaemon struct {
	RESTDaemon *restDaemon
	RPCDaemon  *rpcDaemon
}

func newCombinedDaemon(rest *restDaemon, rpc *rpcDaemon) *combinedDaemon {
	return &combinedDaemon{
		RESTDaemon: rest,
		RPCDaemon:  rpc,
	}
}

func (d *combinedDaemon) Start(s service.Service) error {
	catcher := &erc.Collector{}
	catcher.Add(d.RPCDaemon.Start(s))
	catcher.Add(d.RESTDaemon.Start(s))
	return catcher.Resolve()
}

func (d *combinedDaemon) Stop(s service.Service) error {
	catcher := &erc.Collector{}
	catcher.Add(d.RPCDaemon.Stop(s))
	catcher.Add(d.RESTDaemon.Stop(s))
	return catcher.Resolve()
}
