package lifecycle

// ProcessTree owns a process tree for the lifetime of an app-owned process.
type ProcessTree interface {
	// Close terminates all processes in the tree.
	Close() error
	// PIDs returns the process IDs of all members currently in the tree.
	// Returns nil if the tree does not support enumeration.
	PIDs() []int
}
