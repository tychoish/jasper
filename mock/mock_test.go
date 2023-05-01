package mock

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
)

func TestMockInterfaces(t *testing.T) {
	_, ok := jasper.Manager(&Manager{}).(*Manager)
	check.True(t, ok)

	_, ok = jasper.Process(&Process{}).(*Process)
	check.True(t, ok)
}
