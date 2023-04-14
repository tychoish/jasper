package options

import (
	"context"
	"errors"

	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

// MockSender is a simple mock implementation of the grip/send.Sender
// interface.
// TODO: Remove this mock sender with EVG-13443.
type MockSender struct {
	send.Base
	Closed bool
}

// NewMockSender returns a MockSender with the given name.
func NewMockSender(name string) *MockSender {
	b := &MockSender{}
	b.SetName(name)
	return b
}

// Send noops.
func (*MockSender) Send(_ message.Composer) {}

// Flush noops.
func (*MockSender) Flush(_ context.Context) error { return nil }

// Close will fail if Closed is already set to false.
func (s *MockSender) Close() error {
	if s.Closed {
		return errors.New("mock sender already closed")
	}
	s.Closed = true

	return nil
}
