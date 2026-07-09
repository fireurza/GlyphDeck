package storage

import (
	"path/filepath"
	"testing"
)

func TestDefaultDBPathUsesRepoLocalDataDirByDefault(t *testing.T) {
	t.Setenv("GLYPHDECK_DATA_DIR", "")

	if got, want := DefaultDBPath(), filepath.Join(".glyphdeck", "glyphdeck.db"); got != want {
		t.Fatalf("DefaultDBPath() = %q, want %q", got, want)
	}
}

func TestDefaultDBPathUsesDataDirOverride(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "glyphdeck-data")
	t.Setenv("GLYPHDECK_DATA_DIR", dataDir)

	if got, want := DefaultDBPath(), filepath.Join(dataDir, "glyphdeck.db"); got != want {
		t.Fatalf("DefaultDBPath() = %q, want %q", got, want)
	}
}
