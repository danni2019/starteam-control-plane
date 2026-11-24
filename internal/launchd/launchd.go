package launchd

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/danni2019/starteam-control-plane/internal/paths"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type Manager struct {
	Paths  paths.Paths
	UID    int
	Runner CommandRunner
}

type PlistOptions struct {
	DaemonPath  string
	Workspace   string
	ConfigPath  string
	Listen      string
	Environment map[string]string
}

func New(p paths.Paths) Manager {
	return Manager{Paths: p, UID: os.Getuid(), Runner: ExecRunner{}}
}

func (m Manager) Domain() string {
	return "gui/" + strconv.Itoa(m.UID)
}

func (m Manager) ServiceTarget() string {
	return m.Domain() + "/" + paths.LaunchAgentLabel
}

func (m Manager) WritePlist(options PlistOptions) error {
	if options.DaemonPath == "" || options.Workspace == "" || options.ConfigPath == "" || options.Listen == "" {
		return errors.New("daemon path, workspace, config path, and listen are required")
	}
	if err := m.Paths.Ensure(); err != nil {
		return err
	}
	content, err := renderPlist(options, m.Paths)
	if err != nil {
		return err
	}
	temporary := m.Paths.LaunchAgent + ".tmp"
	if err := os.WriteFile(temporary, content, 0o600); err != nil {
		return fmt.Errorf("write launch agent: %w", err)
	}
	if err := os.Rename(temporary, m.Paths.LaunchAgent); err != nil {
		return fmt.Errorf("install launch agent: %w", err)
	}
	return nil
}

func (m Manager) Start(ctx context.Context) error {
	if _, err := m.Runner.Run(ctx, "launchctl", "enable", m.ServiceTarget()); err != nil {
		return fmt.Errorf("enable launch agent: %w", err)
	}
	loaded, err := m.Loaded(ctx)
	if err != nil {
		return err
	}
	if !loaded {
		output, err := m.Runner.Run(ctx, "launchctl", "bootstrap", m.Domain(), m.Paths.LaunchAgent)
		if err != nil {
			return commandError("bootstrap launch agent", output, err)
		}
		return nil
	}
	output, err := m.Runner.Run(ctx, "launchctl", "kickstart", "-k", m.ServiceTarget())
	if err != nil {
		return commandError("kickstart launch agent", output, err)
	}
	return nil
}

func (m Manager) Stop(ctx context.Context) error {
	if _, err := m.Runner.Run(ctx, "launchctl", "disable", m.ServiceTarget()); err != nil {
		return fmt.Errorf("disable launch agent: %w", err)
	}
	loaded, err := m.Loaded(ctx)
	if err != nil {
		return err
	}
	if !loaded {
		return nil
	}
	output, err := m.Runner.Run(ctx, "launchctl", "bootout", m.ServiceTarget())
	if err != nil {
		return commandError("bootout launch agent", output, err)
	}
	return nil
}

func (m Manager) Loaded(ctx context.Context) (bool, error) {
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := m.Runner.Run(probe, "launchctl", "print", m.ServiceTarget())
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, fmt.Errorf("inspect launch agent: %w", err)
}

func commandError(action string, output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %s: %w", action, message, err)
}

func renderPlist(options PlistOptions, p paths.Paths) ([]byte, error) {
	tmpl, err := template.New("plist").Funcs(template.FuncMap{
		"xml": func(value string) string {
			var buffer bytes.Buffer
			_ = xml.EscapeText(&buffer, []byte(value))
			return buffer.String()
		},
	}).Parse(plistTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse launch agent template: %w", err)
	}
	data := struct {
		PlistOptions
		StdoutPath string
		StderrPath string
		SearchPath string
	}{
		PlistOptions: options,
		StdoutPath:   filepath.Join(p.Logs, "starteamd.log"),
		StderrPath:   filepath.Join(p.Logs, "starteamd.error.log"),
		SearchPath:   filepath.Join(p.Home, ".local", "bin") + ":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin",
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return nil, fmt.Errorf("render launch agent: %w", err)
	}
	return output.Bytes(), nil
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.starteam.control-plane</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{xml .DaemonPath}}</string>
    <string>--workspace</string>
    <string>{{xml .Workspace}}</string>
    <string>--config</string>
    <string>{{xml .ConfigPath}}</string>
    <string>--listen</string>
    <string>{{xml .Listen}}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>ThrottleInterval</key>
  <integer>5</integer>
  <key>ProcessType</key>
  <string>Background</string>
  <key>WorkingDirectory</key>
  <string>{{xml .Workspace}}</string>
  <key>StandardOutPath</key>
  <string>{{xml .StdoutPath}}</string>
  <key>StandardErrorPath</key>
  <string>{{xml .StderrPath}}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>{{xml .SearchPath}}</string>
    {{range $key, $value := .Environment}}
    <key>{{xml $key}}</key>
    <string>{{xml $value}}</string>
    {{end}}
  </dict>
</dict>
</plist>
`
