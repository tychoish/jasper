package jasper_test

import (
	"context"
	"testing"

	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testcases"
	"github.com/tychoish/jasper/testutil"
)

func TestProcessImplementations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for pname, makeProc := range testcases.ProcessConstructors() {
		t.Run(pname, func(t *testing.T) {
			for optsTestName, modifyOpts := range map[string]func(*options.Create) *options.Create{
				"Local": func(opts *options.Create) *options.Create { return opts },
			} {
				t.Run(optsTestName, func(t *testing.T) {
					for testName, testCase := range testcases.ProcessCases() {
						t.Run(testName, func(t *testing.T) {
							tctx, cancel := context.WithTimeout(ctx, testutil.ProcessTestTimeout)
							defer cancel()

							opts := &options.Create{Args: []string{"ls"}}
							opts = modifyOpts(opts)
							testCase(tctx, t, opts, makeProc)
						})
					}
				})
			}
		})
	}
}
