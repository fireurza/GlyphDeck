package terminal

import (
	"testing"
)

type mockProcessTree struct {
	closed int
}

func (m *mockProcessTree) Close() error {
	m.closed++
	return nil
}

func (m *mockProcessTree) PIDs() []int { return nil }

func TestCloseTerminatesTrackedProcess(t *testing.T) {
	tree := &mockProcessTree{}
	mgr := &Manager{
		terminals: map[string]*Terminal{
			"term-1": {
				ID:          "term-1",
				processTree: tree,
			},
		},
		terminator: nil,
	}

	if err := mgr.Close("term-1"); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if tree.closed != 1 {
		t.Fatalf("processTree.Close() calls = %d, want 1", tree.closed)
	}
	if status, err := mgr.Status("term-1"); err != nil || status.Running {
		t.Fatalf("Status() = %#v, %v; want closed terminal", status, err)
	}
}

func TestCloseAllTerminatesEveryTrackedProcess(t *testing.T) {
	tree1 := &mockProcessTree{}
	tree2 := &mockProcessTree{}
	mgr := &Manager{
		terminals: map[string]*Terminal{
			"term-1": {ID: "term-1", processTree: tree1},
			"term-2": {ID: "term-2", processTree: tree2},
		},
		terminator: nil,
	}

	if err := mgr.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}
	if tree1.closed != 1 {
		t.Fatalf("tree1.Close() calls = %d, want 1", tree1.closed)
	}
	if tree2.closed != 1 {
		t.Fatalf("tree2.Close() calls = %d, want 1", tree2.closed)
	}
}
