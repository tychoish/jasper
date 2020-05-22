package mock

import (
	"testing"

	"github.com/deciduosity/jasper"
	"github.com/deciduosity/jasper/remote"
	"github.com/stretchr/testify/assert"
)

func TestMockInterfaces(t *testing.T) {
	assert.Implements(t, (*jasper.Manager)(nil), &Manager{})
	assert.Implements(t, (*jasper.Process)(nil), &Process{})
	assert.Implements(t, (*remote.Manager)(nil), &RemoteClient{})
}
