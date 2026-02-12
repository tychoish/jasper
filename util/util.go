package util

import (
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/tychoish/grip/send"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// CloseFunc is a function used to close a service or close the client
// connection to a service.
type CloseFunc func() error

// NewLocalBuffer provides a synchronized read/Write closer.
func NewLocalBuffer(b *bytes.Buffer) *LocalBuffer { return &LocalBuffer{b: b} }

type LocalBuffer struct {
	b *bytes.Buffer
	sync.Mutex
}

func (b *LocalBuffer) Read(p []byte) (n int, err error) {
	b.Lock()
	defer b.Unlock()
	return b.b.Read(p)
}

func (b *LocalBuffer) Write(p []byte) (n int, err error) {
	b.Lock()
	defer b.Unlock()
	return b.b.Write(p)
}

func (b *LocalBuffer) String() string {
	b.Lock()
	defer b.Unlock()
	return b.b.String()
}

func (b *LocalBuffer) Close() error { return nil }

func ConvertWriter(wr io.Writer, err error) send.Sender {
	if err != nil || wr == nil {
		return nil
	}

	return send.MakeWriter(wr)
}
