//go:build !windows

package lifecycle

import "os"

// AttachProcessTree has no platform job-object implementation outside Windows.
func AttachProcessTree(_ *os.Process) (ProcessTree, error) {
	return nil, nil
}
