package mock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tychoish/jasper/x/remote"
)

func TestMockInterfaces(t *testing.T) {
	assert.Implements(t, (*remote.Manager)(nil), &RemoteClient{})
}
