package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/danni2019/starteam-control-plane/internal/gateway"
	"github.com/danni2019/starteam-control-plane/internal/launchd"
	"github.com/danni2019/starteam-control-plane/internal/paths"
)

type config struct {
	Primary string `json:"primary"`
	Standby string `json:"standby"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && args[0] == "demo" {
		return demo()
	}
	if len(args) > 0 && args[0] == "write-plist" {
		return writePlist(args[1:])
	}
	flags := flag.NewFlagSet("starteam-control", flag.ContinueOnError)
	configPath := flags.String("config", "", "JSON configuration path")
	listen := flags.String("listen", "127.0.0.1:8765", "loopback listen address")
	workspace := flags.String("workspace", "", "resolve a relative config path from this directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("listen address must use a literal loopback IP and port")
	}
	if *configPath == "" {
		return errors.New("use demo or supply --config")
	}
	path := *configPath
	if *workspace != "" && !filepath.IsAbs(path) {
		path = filepath.Join(*workspace, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	control, err := gateway.New(cfg.Primary, cfg.Standby)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: *listen, Handler: control, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second}
	return server.ListenAndServe()
}

func demo() error {
	var healthy atomic.Bool
	healthy.Store(true)
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer primary.Close()
	standby := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer standby.Close()
	control, err := gateway.New(primary.URL, standby.URL)
	if err != nil {
		return err
	}
	first := control.Status(context.Background())
	healthy.Store(false)
	second := control.Status(context.Background())
	if first.Active != "primary" || second.Active != "standby" {
		return fmt.Errorf("expected primary then standby, got %s and %s", first.Active, second.Active)
	}
	return json.NewEncoder(os.Stdout).Encode([]gateway.Status{first, second})
}

func writePlist(args []string) error {
	flags := flag.NewFlagSet("write-plist", flag.ContinueOnError)
	root := flags.String("root", "", "directory for the rendered launch agent")
	binary := flags.String("binary", "", "absolute path to the built control-plane binary")
	configPath := flags.String("config", "", "absolute path to the backend configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *root == "" || !filepath.IsAbs(*binary) || !filepath.IsAbs(*configPath) {
		return errors.New("supply --root and absolute --binary and --config paths")
	}
	p := paths.ForRuntimeRoot(*root)
	manager := launchd.New(p)
	return manager.WritePlist(launchd.PlistOptions{
		DaemonPath: *binary, Workspace: filepath.Dir(*configPath), ConfigPath: *configPath, Listen: "127.0.0.1:8765",
	})
}
