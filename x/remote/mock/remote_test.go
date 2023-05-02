package mock

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/jasper/x/remote"
)

func TestMockInterfaces(t *testing.T) {
	_, ok := remote.Manager(&RemoteClient{}).(*RemoteClient)
	assert.True(t, ok)
}
