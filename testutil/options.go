package testutil

import (
	"fmt"
	"time"

	"github.com/deciduosity/jasper/options"
)

// YesCreateOpts creates the options to run the "yes" command for the given
// duration.
func YesCreateOpts(timeout time.Duration) *options.Create {
	return &options.Create{Args: []string{"yes"}, Timeout: timeout}
}

// TrueCreateOpts creates the options to run the "true" command.
func TrueCreateOpts() *options.Create {
	return &options.Create{
		Args: []string{"true"},
	}
}

// FalseCreateOpts creates the options to run the "false" command.
func FalseCreateOpts() *options.Create {
	return &options.Create{
		Args: []string{"false"},
	}
}

// SleepCreateOpts creates the options to run the "sleep" command for the give
// nnumber of seconds.
func SleepCreateOpts(num int) *options.Create {
	return &options.Create{
		Args: []string{"sleep", fmt.Sprint(num)},
	}
}

// OptsModify functions mutate creation options for tests.
type OptsModify func(*options.Create)
