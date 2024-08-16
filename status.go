package pm

const (
	StatusUnknown Status = iota
	StatusStarting
	StatusRunning
	StatusStopping
	StatusStopped
	StatusError
	StatusCrashLoop
)

type Status uint32

func (s Status) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	case StatusCrashLoop:
		return "crashloop"
	default:
		return "unknown"
	}
}
