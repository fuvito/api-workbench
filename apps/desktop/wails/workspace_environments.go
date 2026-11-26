package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/fuvito/api-workbench/packages/core/serialization"
)

// Environment files and the active environment.

type EnvironmentInfoIO struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type EnvironmentVariableIO struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
}

type EnvironmentIO struct {
	Path      string                  `json:"path"`
	Name      string                  `json:"name"`
	Variables []EnvironmentVariableIO `json:"variables"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
}

func (w *WorkspaceService) ActiveEnvironment() *domain.Environment {
	return w.activeEnv
}

// ListEnvironments returns all environment files found in workspacePath/environments/.
func (w *WorkspaceService) ListEnvironments() []EnvironmentInfoIO {
	if w.workspacePath == "" {
		return nil
	}
	envDir := filepath.Join(w.workspacePath, "environments")
	entries, err := os.ReadDir(envDir)
	if err != nil {
		return nil
	}

	var result []EnvironmentInfoIO
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isDataFile(name) {
			continue
		}
		path := filepath.Join(envDir, name)
		displayName := strings.TrimSuffix(name, filepath.Ext(name))
		// Try to read the declared name from the file
		if data, err := os.ReadFile(path); err == nil {
			if env, err := serialization.ParseEnvironmentFile(data, name); err == nil && env.Name != "" {
				displayName = env.Name
			}
		}
		result = append(result, EnvironmentInfoIO{Name: displayName, Path: path})
	}
	return result
}

// SelectEnvironment sets the active environment by file path.
func (w *WorkspaceService) SelectEnvironment(path string) error {
	if path == "" {
		w.activeEnv = nil
		w.activeEnvPath = ""
		w.activeEnvSource = envSourceNone
		w.rememberEnvironment("")
		w.persistConfig()
		return nil
	}
	env, err := w.loadEnvFile(path)
	if err != nil {
		return err
	}
	w.activeEnv = env
	w.activeEnvPath = path
	w.activeEnvSource = envSourceChosen
	w.rememberEnvironment(path)
	w.persistConfig()
	return nil
}

// rememberEnvironment records the environment chosen for the open workspace
// (relative to it, so a moved workspace keeps its choice). Temporary demo and
// conformance workspaces are not remembered.
func (w *WorkspaceService) rememberEnvironment(envPath string) {
	if w.workspacePath == "" || isTempWorkspace(w.workspacePath) {
		return
	}
	if envPath == "" {
		delete(w.workspaceEnvs, w.workspacePath)
		return
	}
	rel, err := filepath.Rel(w.workspacePath, envPath)
	if err != nil || outsideRel(rel) {
		return
	}
	if w.workspaceEnvs == nil {
		w.workspaceEnvs = map[string]string{}
	}
	w.workspaceEnvs[w.workspacePath] = rel
}

// GetActiveEnvironment returns info about the currently selected environment (or nil).
func (w *WorkspaceService) GetActiveEnvironment() *EnvironmentInfoIO {
	if w.activeEnv == nil {
		return nil
	}
	return &EnvironmentInfoIO{Name: w.activeEnv.Name, Path: w.activeEnvPath}
}

// GetEnvironment loads a full environment (including variable values) for editing.
func (w *WorkspaceService) GetEnvironment(path string) (*EnvironmentIO, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	env, err := serialization.ParseEnvironmentFile(data, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	return environmentToIO(env, path), nil
}

func environmentToIO(env *domain.Environment, path string) *EnvironmentIO {
	out := &EnvironmentIO{Path: path, Name: env.Name, Extensions: string(env.Extensions)}
	for _, v := range env.Variables {
		out.Variables = append(out.Variables, EnvironmentVariableIO{Name: v.Name, Value: v.Value, Secret: v.Secret, Extensions: string(v.Extensions)})
	}
	return out
}

// ParseEnvironmentYAML reads environment text for the editor's form, as loading the file would.
func (w *WorkspaceService) ParseEnvironmentYAML(content string) (*EnvironmentIO, error) {
	env, err := serialization.ParseEnvironment([]byte(content), "yaml")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(env.Name) == "" {
		return nil, fmt.Errorf("missing required field: name")
	}
	return environmentToIO(env, ""), nil
}

// SaveEnvironment creates or updates an environment file.
// If env.Path is empty, a new file is created in the workspace environments directory.
func (w *WorkspaceService) SaveEnvironment(env EnvironmentIO) (*EnvironmentInfoIO, error) {
	path := env.Path
	if path == "" {
		if w.workspacePath == "" {
			return nil, fmt.Errorf("no workspace open — open or create a workspace first")
		}
		envDir := filepath.Join(w.workspacePath, "environments")
		if err := os.MkdirAll(envDir, 0755); err != nil {
			return nil, err
		}
		name := sanitizeFilename(env.Name)
		if name == "" {
			name = "environment"
		}
		path = filepath.Join(envDir, name+".yaml")
	}

	domEnv := &domain.Environment{
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          env.Name,
		Extensions:    domain.Extensions(env.Extensions),
	}
	for _, v := range env.Variables {
		domEnv.Variables = append(domEnv.Variables, domain.EnvironmentVariable{
			Name:   v.Name,
			Value:  v.Value,
			Secret: v.Secret,

			Extensions: domain.Extensions(v.Extensions),
		})
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		ext = "yaml"
		path += ".yaml"
	}
	data, err := serialization.WriteEnvironment(domEnv, ext)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(path, data); err != nil {
		return nil, err
	}
	if path == w.activeEnvPath {
		w.activeEnv = domEnv
	}
	return &EnvironmentInfoIO{Name: env.Name, Path: path}, nil
}

// ValidateEnvironmentYAML parses YAML content as an environment file.
// Returns "" on success, or a human-readable error on failure.
func (w *WorkspaceService) ValidateEnvironmentYAML(content string) string {
	_, err := serialization.ParseEnvironmentFile([]byte(content), "env.yaml")
	if err != nil {
		return err.Error()
	}
	return ""
}

// GetEnvironmentRaw returns the raw YAML/JSON text of an environment file for direct editing.
func (w *WorkspaceService) GetEnvironmentRaw(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

// SaveEnvironmentRaw validates that content parses as a valid environment file, writes it
// atomically, and returns the updated environment info.
func (w *WorkspaceService) SaveEnvironmentRaw(path, content string) (*EnvironmentInfoIO, error) {
	if path == "" {
		if w.workspacePath == "" {
			return nil, fmt.Errorf("no workspace open — open or create a workspace first")
		}
		return nil, fmt.Errorf("cannot save raw YAML without a file path — save via the form first")
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		ext = "yaml"
	}

	env, err := serialization.ParseEnvironmentFile([]byte(content), filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("invalid environment YAML: %w", err)
	}
	if err := atomicWrite(path, []byte(content)); err != nil {
		return nil, err
	}

	info := &EnvironmentInfoIO{Path: path, Name: env.Name}
	if path == w.activeEnvPath {
		w.activeEnv = env
	}
	return info, nil
}

// DeleteEnvironment removes an environment file.
// If it was the active environment, the active selection is cleared.
func (w *WorkspaceService) DeleteEnvironment(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting %s: %w", path, err)
	}
	if path == w.activeEnvPath {
		w.activeEnv = nil
		w.activeEnvPath = ""
		w.persistConfig()
	}
	return nil
}

func (w *WorkspaceService) loadEnvFile(path string) (*domain.Environment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return serialization.ParseEnvironmentFile(data, filepath.Base(path))
}
