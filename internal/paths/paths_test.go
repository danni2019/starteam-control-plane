package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForHomeKeepsRuntimeOutsideWorkspace(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "test")
	got := ForHome(home)
	if got.Database != filepath.Join(home, "Library", "Application Support", "starTeam", "control.sqlite3") {
		t.Fatalf("database path = %q", got.Database)
	}
	if got.LaunchAgent != filepath.Join(home, "Library", "LaunchAgents", "com.starteam.runtime.plist") {
		t.Fatalf("launch agent path = %q", got.LaunchAgent)
	}
}

func TestCurrentHonorsExplicitRuntimeRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STARTEAM_RUNTIME_ROOT", root)
	got, err := Current()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "data", "control.sqlite3")
	if got.Database != want {
		t.Fatalf("database = %q, want %q", got.Database, want)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
}
