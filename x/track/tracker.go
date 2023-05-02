package track

import "github.com/tychoish/jasper"

// processTrackerBase provides convenience no-op implementations of the
// ProcessTracker interface.
type processTrackerBase struct{ Name string }

func (*processTrackerBase) Add(jasper.ProcessInfo) error { return nil }
func (*processTrackerBase) Cleanup() error               { return nil }
