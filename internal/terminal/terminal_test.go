package terminal

import (
	"os"
	"os/exec"
	"testing"
)

func TestCloseTerminatesTrackedProcess(t *testing.T) {
	terminated := 0
	mgr := &Manager{
		terminals: map[string]*Terminal{
			"term-1": {
				ID:  "term-1",
				cmd: &exec.Cmd{Process: &os.Process{Pid: 4321}},
			},
		},
		terminator: func(process *os.Process) error {
			terminated++
			if process.Pid != 4321 {
				t.Fatalf("process PID = %d, want 4321", process.Pid)
			}
			return nil
		},
	}

	if err := mgr.Close("term-1"); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if terminated != 1 {
		t.Fatalf("termination calls = %d, want 1", terminated)
	}
	if status, err := mgr.Status("term-1"); err != nil || status.Running {
		t.Fatalf("Status() = %#v, %v; want closed terminal", status, err)
	}
}

func TestCloseAllTerminatesEveryTrackedProcess(t *testing.T) {
	terminated := 0
	mgr := &Manager{
		terminals: map[string]*Terminal{
			"term-1": {ID: "term-1", cmd: &exec.Cmd{Process: &os.Process{Pid: 1}}},
			"term-2": {ID: "term-2", cmd: &exec.Cmd{Process: &os.Process{Pid: 2}}},
		},
		terminator: func(*os.Process) error {
			terminated++
			return nil
		},
	}

	if err := mgr.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}
	if terminated != 2 {
		t.Fatalf("termination calls = %d, want 2", terminated)
	}
}
