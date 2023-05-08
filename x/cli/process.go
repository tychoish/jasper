package cli

import (
	"context"
	"fmt"
	"syscall"

	"github.com/tychoish/jasper/x/remote"
	"github.com/urfave/cli/v2"
)

// Constants representing the Jasper Process interface as CLI commands.
const (
	ProcessCommand                 = "process"
	InfoCommand                    = "info"
	CompleteCommand                = "complete"
	RegisterSignalTriggerIDCommand = "register-signal-trigger-id"
	RespawnCommand                 = "respawn"
	RunningCommand                 = "running"
	SignalCommand                  = "signal"
	TagCommand                     = "tag"
	GetTagsCommand                 = "get-tags"
	ResetTagsCommand               = "reset-tags"
	WaitCommand                    = "wait"
)

// Process creates a cli.Command that interfaces with a Jasper process. Due to
// it being a remote process, there is no CLI equivalent of of RegisterTrigger
// or RegisterSignalTrigger.
func Process() *cli.Command {
	return &cli.Command{
		Name: ProcessCommand,
		Subcommands: []*cli.Command{
			processInfo(),
			processRunning(),
			processComplete(),
			processTag(),
			processGetTags(),
			processResetTags(),
			processRespawn(),
			processRegisterSignalTriggerID(),
			processSignal(),
			processWait(),
		},
	}
}

func processInfo() *cli.Command {
	return &cli.Command{
		Name:   InfoCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &IDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return &InfoResponse{OutcomeResponse: *makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))}
				}
				return &InfoResponse{Info: proc.Info(ctx), OutcomeResponse: *makeOutcomeResponse(nil)}
			})
		},
	}
}

func processRunning() *cli.Command {
	return &cli.Command{
		Name:   RunningCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &IDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return &RunningResponse{OutcomeResponse: *makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))}
				}
				return &RunningResponse{Running: proc.Running(ctx), OutcomeResponse: *makeOutcomeResponse(nil)}
			})
		},
	}
}

func processComplete() *cli.Command {
	return &cli.Command{
		Name:   CompleteCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &IDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return &CompleteResponse{OutcomeResponse: *makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))}
				}
				return &CompleteResponse{Complete: proc.Complete(ctx), OutcomeResponse: *makeOutcomeResponse(nil)}
			})
		},
	}
}

func processSignal() *cli.Command {
	return &cli.Command{
		Name:   SignalCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &SignalInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))
				}
				return makeOutcomeResponse(proc.Signal(ctx, syscall.Signal(input.Signal)))
			})
		},
	}
}

func processWait() *cli.Command {
	return &cli.Command{
		Name:   WaitCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &IDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return &WaitResponse{OutcomeResponse: *makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))}
				}
				exitCode, err := proc.Wait(ctx)
				if err != nil {
					return &WaitResponse{ExitCode: exitCode, Error: err.Error(), OutcomeResponse: *makeOutcomeResponse(nil)}
				}
				return &WaitResponse{ExitCode: exitCode, OutcomeResponse: *makeOutcomeResponse(nil)}
			})
		},
	}
}

func processRespawn() *cli.Command {
	return &cli.Command{
		Name:   RespawnCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &IDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return &InfoResponse{OutcomeResponse: *makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))}
				}
				newProc, err := proc.Respawn(ctx)
				if err != nil {
					return &InfoResponse{OutcomeResponse: *makeOutcomeResponse(fmt.Errorf("error respawning process with id '%s': %w", input.ID, err))}
				}
				return &InfoResponse{Info: newProc.Info(ctx), OutcomeResponse: *makeOutcomeResponse(nil)}
			})
		},
	}
}

func processRegisterSignalTriggerID() *cli.Command {
	return &cli.Command{
		Name:   RegisterSignalTriggerIDCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &SignalTriggerIDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))
				}
				if err := proc.RegisterSignalTriggerID(ctx, input.SignalTriggerID); err != nil {
					return makeOutcomeResponse(fmt.Errorf("couldn't register signal trigger with id '%s' on process with id '%s': %w", input.SignalTriggerID, input.ID, err))
				}
				return makeOutcomeResponse(nil)
			})
		},
	}
}

func processTag() *cli.Command {
	return &cli.Command{
		Name:   TagCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &TagIDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))
				}
				proc.Tag(input.Tag)
				return makeOutcomeResponse(nil)
			})
		},
	}
}

func processGetTags() *cli.Command {
	return &cli.Command{
		Name:   GetTagsCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &IDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return &TagsResponse{OutcomeResponse: *makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))}
				}
				return &TagsResponse{Tags: proc.GetTags(), OutcomeResponse: *makeOutcomeResponse(nil)}
			})
		},
	}
}

func processResetTags() *cli.Command {
	return &cli.Command{
		Name:   ResetTagsCommand,
		Flags:  clientFlags(),
		Before: clientBefore(),
		Action: func(c *cli.Context) error {
			input := &IDInput{}
			return doPassthroughInputOutput(c, input, func(ctx context.Context, client remote.Manager) interface{} {
				proc, err := client.Get(ctx, input.ID)
				if err != nil {
					return makeOutcomeResponse(fmt.Errorf("error finding process with id '%s': %w", input.ID, err))
				}
				proc.ResetTags()
				return makeOutcomeResponse(nil)
			})
		},
	}
}
