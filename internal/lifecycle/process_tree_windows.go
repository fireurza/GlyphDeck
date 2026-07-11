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

// PIDs returns the process IDs of all members currently in the Job Object.
func (tree *windowsProcessTree) PIDs() []int {
	if tree == nil || tree.job == 0 || tree.job == windows.InvalidHandle {
		return nil
	}
	const infoClass uint32 = 3 // JobObjectBasicProcessIdList
	buf := make([]byte, 4096)
	var retlen uint32
	err := windows.QueryInformationJobObject(
		tree.job,
		int32(infoClass),
		uintptr(unsafe.Pointer(&buf[0])),
		uint32(len(buf)),
		&retlen,
	)
	if err != nil {
		return nil
	}
	if len(buf) < 4 || retlen < 4 {
		return nil
	}
	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	if count == 0 {
		return nil
	}
	pids := make([]int, 0, count)
	const pidOffset = 4
	const pidSize = 8
	for i := uint32(0); i < count && int(pidOffset+i*pidSize+pidSize) <= len(buf); i++ {
		ptr := unsafe.Pointer(&buf[pidOffset+i*pidSize])
		pid := int(*(*uintptr)(ptr))
		if pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}
