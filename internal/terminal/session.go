package terminal

import (
	"io"
	"os"
)

// termSession abstracts a platform-specific PTY or pipe shell session.
// Callers drive the session through the Manager; the backend manages OS resources.
type termSession interface {
	// stdin returns the writer connected to the process standard input.
	stdin() io.WriteCloser
	// stdout returns the reader connected to the process standard output (and stderr).
	stdout() io.ReadCloser
	// resize signals a terminal window size change to the backend.
	resize(rows, cols uint16) error
	// process returns the shell process for lifecycle management.
	process() *os.Process
	// wait blocks until the process exits and then releases backend resources.
	wait() error
	// close flushes and closes the input writer to initiate graceful shutdown.
	// The backend frees PTY/pipe resources in wait() after the process exits.
	close() error
}
