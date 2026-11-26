package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// What is remembered between sessions: recent workspaces and each one's environment.

// ---- Config persistence ----

type workspaceConfig struct {
	WorkspacePath    string   `json:"workspacePath"`
	ActiveEnvPath    string   `json:"activeEnvPath"`
	RecentWorkspaces []string `json:"recentWorkspaces,omitempty"`
	// WorkspaceEnvs remembers the selected environment per workspace: workspace
	// path -> environment file path relative to that workspace.
	WorkspaceEnvs map[string]string `json:"workspaceEnvs,omitempty"`
	WindowWidth   int               `json:"windowWidth,omitempty"`
	WindowHeight  int               `json:"windowHeight,omitempty"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".clrsig")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() workspaceConfig {
	p, err := configPath()
	if err != nil {
		return workspaceConfig{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return workspaceConfig{}
	}
	var cfg workspaceConfig
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg workspaceConfig) {
	p, err := configPath()
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = atomicWrite(p, data)
}

// ---- helpers ----

func (w *WorkspaceService) persistConfig() {
	saveConfig(workspaceConfig{
		WorkspacePath:    w.workspacePath,
		ActiveEnvPath:    w.activeEnvPath,
		RecentWorkspaces: w.recentWorkspaces,
		WorkspaceEnvs:    w.workspaceEnvs,
	})
}

func (w *WorkspaceService) addRecentWorkspace(path string) {
	if path == "" || isTempWorkspace(path) {
		return
	}
	seen := make(map[string]bool)
	recent := []string{path}
	seen[path] = true
	for _, p := range w.recentWorkspaces {
		if !seen[p] {
			seen[p] = true
			recent = append(recent, p)
		}
	}
	if len(recent) > 10 {
		recent = recent[:10]
	}
	w.recentWorkspaces = recent
}

// GetRecentWorkspaces returns the list of recently opened workspace paths (most recent first).
func (w *WorkspaceService) GetRecentWorkspaces() []string {
	if len(w.recentWorkspaces) == 0 {
		return []string{}
	}
	return w.recentWorkspaces
}
