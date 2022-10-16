package jasper

import (
	"context"
	"fmt"
	"syscall"
)

// SignalEvent signals the event object represented by the given name.
func SignalEvent(ctx context.Context, name string) error {
	utf16EventName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("failed to convert event name '%s': %w", name, err)
	}

	event, err := OpenEvent(utf16EventName)
	if err != nil {
		return fmt.Errorf("failed to open event '%s': %w", name, err)
	}
	defer CloseHandle(event)

	if err := SetEvent(event); err != nil {
		return fmt.Errorf("failed to signal event '%s': %w", name, err)
	}

	return nil
}
