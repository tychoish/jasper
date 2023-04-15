package executor

// Status represents the current state of a process execution
type Status int

func (s Status) String() string {
	switch s {
	case Unknown:
		return "unknown"
	case Unstarted:
		return "unstarted"
	case Running:
		return "running"
	case Exited:
		return "exited"
	case Closed:
		return "closed"
	default:
		return ""
	}
}

func (s Status) Before(other Status) bool {
	return s < other
}

func (s Status) BeforeInclusive(other Status) bool {
	return s <= other
}

func (s Status) After(other Status) bool {
	return s > other
}

func (s Status) AfterInclusive(other Status) bool {
	return s >= other
}

func (s Status) Between(lower Status, upper Status) bool {
	return lower < s && s < upper
}

func (s Status) BetweenInclusive(lower Status, upper Status) bool {
	return lower <= s && s <= upper
}

const (
	// Unknown means the process is in an unknown or invalid state.
	Unknown Status = iota
	// Unstarted means that the Executor has not yet started running the
	// process.
	Unstarted Status = iota
	// Running means that the Executor has started running the process.
	Running Status = iota
	// Exited means the Executor has finished running the process.
	Exited Status = iota
	// Closed means the Executor has cleaned up its resources and further
	// requests cannot be made.
	Closed Status = iota
)
