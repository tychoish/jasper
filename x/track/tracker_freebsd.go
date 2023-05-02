package Tracker

type freebsdProcessTracker struct {
	*processTrackerBase
}

// New is unimplemented.
func New(name string) (ProcessTracker, error) {
	return &freebsdProcessTracker{processTrackerBase: &processTrackerBase{Name: name}}, nil
}
