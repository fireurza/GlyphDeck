// Package lifecycle manages process cleanup for app-owned child processes.
package lifecycle

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

// TerminateProcessTree stops an app-owned process and, on Windows, its children.
func TerminateProcessTree(process *os.Process) error {
	if process == nil || process.Pid <= 0 {
		return nil
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", taskkillArgs(process.Pid)...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			if killErr := process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				return fmt.Errorf("terminate process tree %d: taskkill: %w; fallback kill: %v", process.Pid, err, killErr)
			}
			return fmt.Errorf("terminate process tree %d: %w", process.Pid, err)
		}
		return nil
	}

	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate process %d: %w", process.Pid, err)
	}
	return nil
}

func taskkillArgs(pid int) []string {
	return []string{"/PID", strconv.Itoa(pid), "/T", "/F"}
}
