package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	LaunchAgentLabel = "com.starteam.runtime"
	DefaultListen    = "127.0.0.1:37640"
)

type Paths struct {
	Home            string
	ApplicationData string
	Logs            string
	ConfigHome      string
	UserConfig      string
	Database        string
	State           string
	LaunchAgent     string
}

func (p Paths) StarPulseStaging() string {
	return filepath.Join(p.ApplicationData, "starpulse", "staging")
}

func ForRuntimeRoot(root string) Paths {
	return Paths{
		Home:            root,
		ApplicationData: filepath.Join(root, "data"),
		Logs:            filepath.Join(root, "logs"),
		ConfigHome:      filepath.Join(root, "config"),
		UserConfig:      filepath.Join(root, "config", "runtime.json"),
		Database:        filepath.Join(root, "data", "control.sqlite3"),
		State:           filepath.Join(root, "data", "runtime.json"),
		LaunchAgent:     filepath.Join(root, "LaunchAgents", LaunchAgentLabel+".plist"),
	}
}

func ForHome(home string) Paths {
	return Paths{
		Home:            home,
		ApplicationData: filepath.Join(home, "Library", "Application Support", "starTeam"),
		Logs:            filepath.Join(home, "Library", "Logs", "starTeam"),
		ConfigHome:      filepath.Join(home, ".config", "starteam"),
		UserConfig:      filepath.Join(home, ".config", "starteam", "runtime.json"),
		Database:        filepath.Join(home, "Library", "Application Support", "starTeam", "control.sqlite3"),
		State:           filepath.Join(home, "Library", "Application Support", "starTeam", "runtime.json"),
		LaunchAgent:     filepath.Join(home, "Library", "LaunchAgents", LaunchAgentLabel+".plist"),
	}
}

func Current() (Paths, error) {
	if root := os.Getenv("STARTEAM_RUNTIME_ROOT"); root != "" {
		absolute, err := filepath.Abs(root)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve STARTEAM_RUNTIME_ROOT: %w", err)
		}
		return ForRuntimeRoot(absolute), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home: %w", err)
	}
	return ForHome(home), nil
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.ApplicationData, p.Logs, p.ConfigHome, filepath.Dir(p.LaunchAgent)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create runtime directory %s: %w", dir, err)
		}
	}
	return nil
}
