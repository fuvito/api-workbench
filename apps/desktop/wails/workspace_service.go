package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---- WorkspaceService ----

type WorkspaceService struct {
	ctx              context.Context
	wailsReady       bool // true once the real Wails ctx is available
	workspacePath    string
	activeEnvPath    string
	activeEnv        *domain.Environment
	recentWorkspaces []string
	workspaceEnvs    map[string]string
	// activeEnvSource says why the active environment is the one it is: "remembered" (restored for
	// this workspace), "chosen" (picked this session), "temporary" (a Demo or Conformance workspace
	// selected its own) or "none". It is derived, never persisted.
	activeEnvSource string
}

const (
	envSourceRemembered = "remembered"
	envSourceChosen     = "chosen"
	envSourceTemporary  = "temporary"
	envSourceNone       = "none"
)

func NewWorkspaceService() *WorkspaceService {
	cfg := loadConfig()
	cleanupStaleTempWorkspaces()
	// The app always starts on the landing page: the last workspace is only
	// offered in the recent list, never reopened.
	ws := &WorkspaceService{
		ctx:              context.Background(),
		recentWorkspaces: prunedRecentWorkspaces(cfg.RecentWorkspaces),
		workspaceEnvs:    cfg.WorkspaceEnvs,
	}
	ws.persistConfig()
	return ws
}

func isTempWorkspace(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, demoWorkspacePrefix) || strings.HasPrefix(base, conformanceWorkspacePrefix)
}

// prunedRecentWorkspaces drops temporary demo/conformance workspaces and
// folders that no longer exist.
func prunedRecentWorkspaces(in []string) []string {
	var out []string
	for _, p := range in {
		if p == "" || isTempWorkspace(p) {
			continue
		}
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, p)
	}
	return out
}

// cleanupStaleTempWorkspaces removes demo and conformance temp workspaces left behind by a session that
// ended without stopping its server, but not one that another running copy of the app has open.
func cleanupStaleTempWorkspaces() {
	removeStaleTempWorkspaces(os.TempDir(), "", demoWorkspacePrefix, conformanceWorkspacePrefix)
}

func (w *WorkspaceService) setContext(ctx context.Context) {
	w.ctx = ctx
	w.wailsReady = true
}

// syncTitle updates the native window title to reflect the current workspace.
func (w *WorkspaceService) syncTitle() {
	if !w.wailsReady {
		return
	}
	if w.workspacePath == "" {
		runtime.WindowSetTitle(w.ctx, "Clear Signal")
	} else {
		runtime.WindowSetTitle(w.ctx, "Clear Signal — "+filepath.Base(w.workspacePath))
	}
}

// openWorkspaceDirect sets the workspace path directly without a dialog.
// It also auto-selects the environments/local.yaml if present.
// Does not persist to config (temp demo workspace should not pollute saved state).
func (w *WorkspaceService) openWorkspaceDirect(path string) {
	w.workspacePath = path
	w.syncTitle()
	w.activeEnvSource = envSourceNone
	envPath := filepath.Join(path, "environments", "local.yaml")
	if _, err := os.Stat(envPath); err == nil {
		w.activeEnvPath = envPath
		if env, err := w.loadEnvFile(envPath); err == nil {
			w.activeEnv = env
			w.activeEnvSource = envSourceTemporary
		}
	}
}

// ---- Wails-bound methods ----

// OpenWorkspace opens a directory picker dialog and sets the workspace.
func (w *WorkspaceService) OpenWorkspace() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(w.ctx, runtime.OpenDialogOptions{
		Title: "Open Workspace Folder",
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return w.workspacePath, nil // user cancelled
	}
	w.enterWorkspace(dir)
	w.addRecentWorkspace(dir)
	w.persistConfig()
	w.syncTitle()
	return dir, nil
}

// GetWorkspacePath returns the current workspace directory path.
func (w *WorkspaceService) GetWorkspacePath() string {
	return w.workspacePath
}

// CloseWorkspace clears the current workspace and persists the change.
func (w *WorkspaceService) CloseWorkspace() {
	w.workspacePath = ""
	w.activeEnvPath = ""
	w.activeEnv = nil
	w.activeEnvSource = ""
	w.persistConfig()
	w.syncTitle()
}

// enterWorkspace makes path the open workspace and selects the environment that
// was last chosen for it. The previous workspace's environment never carries over.
func (w *WorkspaceService) enterWorkspace(path string) {
	w.workspacePath = path
	w.activeEnv = nil
	w.activeEnvPath = ""
	w.activeEnvSource = envSourceNone
	rel, ok := w.workspaceEnvs[path]
	if !ok {
		return
	}
	full := filepath.Join(path, rel)
	if env, err := w.loadEnvFile(full); err == nil {
		w.activeEnv = env
		w.activeEnvPath = full
		w.activeEnvSource = envSourceRemembered
	}
}

// GetCloseReminder says what closing the open workspace will remember, for the close confirmation.
func (w *WorkspaceService) GetCloseReminder() string {
	switch {
	case w.workspacePath == "":
		return ""
	case isTempWorkspace(w.workspacePath):
		return "This is a temporary workspace: it is removed when its server stops and nothing is remembered."
	case w.activeEnv != nil && w.activeEnvPath != "":
		name := w.activeEnv.Name
		if name == "" {
			name = filepath.Base(w.activeEnvPath)
		}
		return "The environment \"" + name + "\" will be remembered for this workspace."
	}
	return "No environment will be remembered for this workspace."
}

// OpenWorkspaceByPath opens a workspace directly by path without a dialog.
func (w *WorkspaceService) OpenWorkspaceByPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %s", path)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", path)
	}
	w.enterWorkspace(path)
	w.addRecentWorkspace(path)
	w.persistConfig()
	w.syncTitle()
	return path, nil
}

// CreateExampleWorkspace creates an example workspace structure in the given directory.
// Called once when the user creates a new workspace.
func createExampleWorkspace(dir string) error {
	for _, sub := range []string{"environments", "requests"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return err
		}
	}

	env := `schemaVersion: "1"
name: Local Dev
variables:
  - name: baseUrl
    value: http://localhost:8080
  - name: authToken
    value: ""
    secret: true
`
	req := `schemaVersion: "1"
title: Health Check
method: GET
url: "{{baseUrl}}/health"
headers:
  - name: Accept
    value: application/json
`
	writeFile := func(path, content string) error {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.WriteString(f, content)
		return err
	}

	// Gitignore environment files (secrets) and the response cache.
	if err := writeFile(filepath.Join(dir, ".gitignore"), ".clrsigcache/\n"); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "environments", ".gitignore"), "*\n"); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "environments", "local.yaml"), env); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "requests", "health-check.yaml"), req); err != nil {
		return err
	}
	return nil
}

// NewWorkspace opens a dialog to pick or create a directory and initialises workspace structure.
func (w *WorkspaceService) NewWorkspace() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(w.ctx, runtime.OpenDialogOptions{
		Title: "Choose Workspace Folder (new or existing)",
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return w.workspacePath, nil
	}
	// Create example structure only if directory is empty
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		if err := createExampleWorkspace(dir); err != nil {
			log.Printf("creating example workspace: %v", err)
		}
	}
	w.enterWorkspace(dir)
	w.addRecentWorkspace(dir)
	w.persistConfig()
	w.syncTitle()
	return dir, nil
}
