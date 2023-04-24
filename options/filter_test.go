package options

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
)

func TestFilters(t *testing.T) {
	t.Run("ConstantsValidate", func(t *testing.T) {
		for _, f := range []Filter{Running, Terminated, All, Failed, Successful} {
			check.NotError(t, f.Validate())
		}
	})
	t.Run("ConstantEquavalentsValidate", func(t *testing.T) {
		for _, f := range []Filter{"running", "terminated", "all", "failed", "successful"} {
			check.NotError(t, f.Validate())
		}
	})
	t.Run("OtherValuesDoNotValidate", func(t *testing.T) {
		for _, f := range []Filter{"", "foo", "terminate", "terminator", "fail"} {
			check.Error(t, f.Validate())
		}
	})
}
