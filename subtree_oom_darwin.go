// +build darwin

package jasper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/tychoish/grip/recovery"
)

func (o *oomTrackerImpl) Clear(ctx context.Context) error {
	sudo, err := isSudo(ctx)
	if err != nil {
		return fmt.Errorf("error checking sudo: %w", err)
	}

	if sudo {
		return exec.CommandContext(ctx, "sudo", "log", "erase", "--all").Run()
	}

	return exec.CommandContext(ctx, "log", "erase", "--all").Run()
}

func (o *oomTrackerImpl) Check(ctx context.Context) error {
	wasOOMKilled, pids, err := analyzeLogs(ctx)
	if err != nil {
		return fmt.Errorf("error searching log: %w", err)
	}
	o.WasOOMKilled = wasOOMKilled
	o.Pids = pids
	return nil
}

func analyzeLogs(ctx context.Context) (bool, []int, error) {
	var cmd *exec.Cmd
	wasOOMKilled := false
	errs := make(chan error)
	sudo, err := isSudo(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("error checking sudo: %w", err)
	}

	if sudo {
		cmd = exec.CommandContext(ctx, "sudo", "log", "show")
	} else {
		cmd = exec.CommandContext(ctx, "log", "show")
	}
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return false, nil, fmt.Errorf("error creating StdoutPipe for log command: %w", err)
	}

	scanner := bufio.NewScanner(cmdReader)
	if err = cmd.Start(); err != nil {
		return false, nil, fmt.Errorf("Error starting log command: %w", err)
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
		if strings.Contains(line, "low swap") {
			wasOOMKilled = true
			if pid, hasPid := getPidFromLog(line); hasPid {
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
