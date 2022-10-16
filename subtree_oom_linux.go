// +build linux

package jasper

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/tychoish/grip/recovery"
)

func (o *oomTrackerImpl) Clear(ctx context.Context) error {
	sudo, err := isSudo(ctx)
	if err != nil {
		return fmt.Errorf("error checking sudo: %w", err)
	}

	if sudo {
		return errors.Wrap(exec.CommandContext(ctx, "sudo", "dmesg", "-c").Run(), "error clearing dmesg")
	}

	return errors.Wrap(exec.CommandContext(ctx, "dmesg", "-c").Run(), "error clearing dmesg")
}

func (o *oomTrackerImpl) Check(ctx context.Context) error {
	wasOOMKilled, pids, err := analyzeDmesg(ctx)
	if err != nil {
		return fmt.Errorf("error searching log: %w", err)
	}
	o.WasOOMKilled = wasOOMKilled
	o.Pids = pids
	return nil
}

func analyzeDmesg(ctx context.Context) (bool, []int, error) {
	var cmd *exec.Cmd
	wasOOMKilled := false
	errs := make(chan error)

	sudo, err := isSudo(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("error checking sudo: %w", err)
	}

	if sudo {
		cmd = exec.CommandContext(ctx, "sudo", "dmesg")
	} else {
		cmd = exec.CommandContext(ctx, "dmesg")
	}
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return false, nil, fmt.Errorf("error creating StdoutPipe for dmesg command: %w", err)
	}

	scanner := bufio.NewScanner(cmdReader)
	if err = cmd.Start(); err != nil {
		return false, nil, fmt.Errorf("Error starting dmesg command: %w", err)
	}

	go func() {
		defer recovery.LogStackTraceAndContinue("log analysis")
		select {
		case <-ctx.Done():
			return
		case errs <- cmd.Wait():
			return
		}
	}()

	pids := []int{}
	for scanner.Scan() {
		line := scanner.Text()
		if dmesgContainsOOMKill(line) {
			wasOOMKilled = true
			if pid, hasPid := getPidFromDmesg(line); hasPid {
				pids = append(pids, pid)
			}
		}
	}

	select {
	case <-ctx.Done():
		return false, nil, errors.New("request cancelled")
	case err = <-errs:
		return wasOOMKilled, pids, fmt.Errorf("Error waiting for dmesg command: %w", err)
	}
}
