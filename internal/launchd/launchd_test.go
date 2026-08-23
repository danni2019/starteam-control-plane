package launchd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"starteam.local/runtime/internal/paths"
)

type call struct {
	name string
	args []string
}

type fakeRunner struct {
	calls       []call
	printLoaded bool
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{name: name, args: append([]string(nil), args...)})
	if len(args) > 0 && args[0] == "print" && !f.printLoaded {
		return nil, &exec.ExitError{}
	}
	return nil, nil
}

func TestWritePlistEscapesPathsAndUsesBackgroundProcess(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	m := Manager{Paths: p, UID: 501, Runner: &fakeRunner{}}
	err := m.WritePlist(PlistOptions{
		DaemonPath:  "/tmp/a&b/starteamd",
		Workspace:   "/tmp/star team",
		ConfigPath:  "/tmp/star team/config/components.yaml",
		Listen:      "127.0.0.1:37640",
		Environment: map[string]string{"STARTEAM_CLOUD_NEWS_URL": "http://127.0.0.1:37641?a&b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p.LaunchAgent)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "/tmp/a&amp;b/starteamd") || !strings.Contains(text, "<string>Background</string>") {
		t.Fatalf("unexpected plist:\n%s", text)
	}
	if !strings.Contains(text, "STARTEAM_CLOUD_NEWS_URL") || !strings.Contains(text, "?a&amp;b") {
		t.Fatalf("cloud environment was not escaped:\n%s", text)
	}
	if filepath.Ext(p.LaunchAgent) != ".plist" {
		t.Fatalf("launch agent path = %q", p.LaunchAgent)
	}
}

func TestStartBootstrapsWhenNotLoaded(t *testing.T) {
	fake := &fakeRunner{}
	m := Manager{Paths: paths.ForHome(t.TempDir()), UID: 501, Runner: fake}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 3 {
		t.Fatalf("calls = %#v", fake.calls)
	}
	if fake.calls[0].args[0] != "enable" || fake.calls[1].args[0] != "print" || fake.calls[2].args[0] != "bootstrap" {
		t.Fatalf("unexpected calls: %#v", fake.calls)
	}
}
