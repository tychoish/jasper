package mock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/remote"
)

func TestMockInterfaces(t *testing.T) {
	assert.Implements(t, (*jasper.Manager)(nil), &Manager{})
	assert.Implements(t, (*jasper.Process)(nil), &Process{})
	assert.Implements(t, (*remote.Manager)(nil), &RemoteClient{})
}
