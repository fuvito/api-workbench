package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/fuvito/api-workbench/packages/core/projectconfig"
)

// checkProjectFile loads the workspace's clrsig.yaml and sees that what it names is there.
func checkProjectFile(t *testing.T, root string, port int) *projectconfig.Config {
	t.Helper()
	cfg, err := projectconfig.Load(filepath.Join(root, "clrsig.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("http://localhost:%d", port); cfg.BaseURL != want {
		t.Errorf("baseUrl %q, want the port the server runs on, %q", cfg.BaseURL, want)
	}
	if cfg.Conform == nil || cfg.Conform.ReadOnly == nil || !*cfg.Conform.ReadOnly {
		t.Error("a sample project is read-only")
	}
	if cfg.Run == nil || len(cfg.Run.Scenarios) == 0 {
		t.Fatal("no scenarios named")
	}
	for _, p := range append([]string{cfg.Spec, cfg.Environment}, cfg.Run.Scenarios...) {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("the project file names %q: %v", p, err)
		}
	}
	return cfg
}

func TestDemoWorkspaceHasAProjectFileWhoseScenariosPass(t *testing.T) {
	isolateHome(t)
	ws := newWS(t, t.TempDir())
	d := NewDemoService(ws)
	res := d.StartDemo()
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	defer d.StopDemo()
	root := ws.GetWorkspacePath()
	cfg := checkProjectFile(t, root, res.Port)

	runner := NewScenarioService(ws, NewSettingsService())
	ran := 0
	for _, folder := range cfg.Run.Scenarios {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(folder)))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			path := filepath.Join(root, filepath.FromSlash(folder), e.Name())
			out := runner.RunScenario(path, filepath.Join(root, filepath.FromSlash(cfg.Environment)), 0)
			ran++
			if out.Error != "" || !out.Passed {
				t.Errorf("%s: clrsig run in the demo workspace would fail: %s", e.Name(), out.Error)
			}
		}
	}
	if ran == 0 {
		t.Fatal("nothing ran")
	}
}

func TestConformanceWorkspaceHasAProjectFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeConformanceWorkspace(dir, 4321); err != nil {
		t.Fatal(err)
	}
	checkProjectFile(t, dir, 4321)
}
