//go:build windows

package lifecycle

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessTree struct {
	job windows.Handle
}

// AttachProcessTree configures a Windows Job Object that kills all members when closed.
func AttachProcessTree(process *os.Process) (ProcessTree, error) {
	if process == nil || process.Pid <= 0 {
		return nil, fmt.Errorf("process is required for tree ownership")
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}
	cleanupJob := true
	defer func() {
		if cleanupJob {
			_ = windows.CloseHandle(job)
		}
	}()

	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		return nil, fmt.Errorf("set job kill-on-close: %w", err)
	}

	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(process.Pid),
	)
	if err != nil {
		return nil, fmt.Errorf("open process %d for job assignment: %w", process.Pid, err)
	}
	defer windows.CloseHandle(processHandle)

	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		return nil, fmt.Errorf("assign process %d to job: %w", process.Pid, err)
	}

	cleanupJob = false
	return &windowsProcessTree{job: job}, nil
}

func (tree *windowsProcessTree) Close() error {
	if tree == nil || tree.job == 0 {
		return nil
	}
	err := windows.CloseHandle(tree.job)
	tree.job = 0
	if err != nil {
		return fmt.Errorf("close process job: %w", err)
	}
	return nil
}
