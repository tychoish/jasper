package mock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tychoish/jasper"
)

func TestMockInterfaces(t *testing.T) {
	assert.Implements(t, (*jasper.Manager)(nil), &Manager{})
	assert.Implements(t, (*jasper.Process)(nil), &Process{})
}
