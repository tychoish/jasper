package track

// TODO

type darwinProcessTracker struct {
	*processTrackerBase
}

// NewProcessTracker.
func New(name string) (ProcessTracker, error) {
	return &darwinProcessTracker{processTrackerBase: &processTrackerBase{Name: name}}, nil
}
