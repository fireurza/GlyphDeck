package terminal

import (
	"os"
	"path/filepath"
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

func FuzzPathIsDescendant(f *testing.F) {
	seeds := []struct{ root, child string }{
		{"/home/user/project", "/home/user/project/src"},
		{"/home/user/project", "/home/user/project"},
		{"/home/user/project", "/home/user/other"},
		{"/home/user/project", "/home/user/../../../etc/passwd"},
		{"/home/user/project", "/home/user/project/.."},
		{"C:\\Users\\proj", "C:\\Users\\proj\\src"},
		{"C:\\Users\\proj", "C:\\Users\\other"},
		{"/home/user/proj", "/home/user/proj-sibling"},
		{"/tmp/a", "/tmp/a/./b"},
	}
	for _, s := range seeds {
		f.Add(s.root, s.child)
	}

	f.Fuzz(func(t *testing.T, root, child string) {
		_ = pathIsDescendant(root, child)
	})
}

func TestIsContainedProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	if !isContained(tmp, tmp) {
		t.Fatal("project root must contain itself")
	}
}

func TestIsContainedValidDescendant(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !isContained(tmp, sub) {
		t.Fatal("valid subdirectory must be contained")
	}
}

func TestIsContainedSiblingWithCommonPrefix(t *testing.T) {
	tmp := t.TempDir()
	sibling := tmp + "-other"
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if isContained(tmp, sibling) {
		t.Fatal("sibling with shared prefix must not be contained")
	}
}

func TestIsContainedTraversal(t *testing.T) {
	tmp := t.TempDir()
	escape := filepath.Join(tmp, "..", "etc")
	if isContained(tmp, escape) {
		t.Fatal("traversal path must not be contained")
	}
}

func TestIsContainedSymlinkEscape(t *testing.T) {
	tmp := t.TempDir()
	link := filepath.Join(tmp, "escape")
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink creation requires privileges or unsupported on this platform")
	}
	if isContained(tmp, link) {
		t.Fatal("symlink escape must not be contained")
	}
}

func TestIsContainedMissingTarget(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "doesnotexist")
	if !isContained(tmp, missing) {
		t.Fatal("non-existent descendant must still be contained")
	}
}

func TestIsContainedDifferentVolume(t *testing.T) {
	if isContained("C:\\Users", "D:\\Other") {
		t.Fatal("different volumes must not be contained")
	}
}

func TestIsContainedNonExistentParent(t *testing.T) {
	child := filepath.Join(t.TempDir(), "sub")
	if isContained("/does/not/exist", child) {
		t.Fatal("non-existent absolute parent and existing child on different volumes should not be contained")
	}
}
