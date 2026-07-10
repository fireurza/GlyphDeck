package lifecycle

// ProcessTree owns a process tree for the lifetime of an app-owned process.
type ProcessTree interface {
	Close() error
}
