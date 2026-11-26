package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/fuvito/api-workbench/packages/core/serialization"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// Request files and the folders that hold them.

// ---- Wails-serializable I/O types ----

type FileNodeIO struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	IsDir    bool         `json:"isDir"`
	Method   string       `json:"method,omitempty"`
	Children []FileNodeIO `json:"children,omitempty"`
}

type RequestFileIO struct {
	SchemaVersion string         `json:"schemaVersion"`
	Title         string         `json:"title"`
	Method        string         `json:"method"`
	URL           string         `json:"url"`
	Headers       []HeaderIO     `json:"headers,omitempty"`
	QueryParams   []QueryParamIO `json:"queryParams,omitempty"`
	Body          *BodyIO        `json:"body,omitempty"`
	Auth          *AuthIO        `json:"auth,omitempty"`
	TimeoutMs     int64          `json:"timeoutMs,omitempty"`
	// NoFollowRedirects is the file's followRedirects: false.
	NoFollowRedirects bool `json:"noFollowRedirects,omitempty"`
	// FilePath is the on-disk path of the loaded file; not persisted to YAML.
	FilePath string `json:"filePath,omitempty"`
	// TargetDir overrides the save directory for new (unsaved) requests.
	TargetDir string `json:"targetDir,omitempty"`
	// Extensions are the item's x- keys, which the editors carry back unread.
	Extensions string `json:"extensions,omitempty"`
}

// ListRequests returns the workspace file tree limited to request files (.yaml/.yml/.json).
func (w *WorkspaceService) ListRequests() []FileNodeIO {
	if w.workspacePath == "" {
		return nil
	}
	requestsDir := filepath.Join(w.workspacePath, "requests")
	if _, err := os.Stat(requestsDir); os.IsNotExist(err) {
		return nil
	}
	nodes, err := buildFileTree(requestsDir)
	if err != nil {
		log.Printf("listing requests: %v", err)
		return nil
	}
	return nodes
}

// LoadRequest reads a request file and returns a RequestFileIO the UI can populate into the editor.
func (w *WorkspaceService) LoadRequest(path string) (*RequestFileIO, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	req, err := serialization.ParseRequestFile(data, filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	out := domainToRequestFileIO(req)
	out.FilePath = path
	return out, nil
}

// SaveRequest writes the request to disk without a dialog.
// The target path is derived from req.Title: {dir}/{slug}.yaml.
// If req.FilePath is set (file already exists), the directory is preserved and
// the file is renamed when the name slug changes. Returns the saved path.
func (w *WorkspaceService) SaveRequest(req RequestFileIO) (string, error) {
	name := strings.TrimSpace(req.Title)
	if name == "" {
		name = "request"
	}
	slug := sanitizeDirName(name)
	if slug == "" {
		slug = "request"
	}

	var targetDir string
	if req.FilePath != "" {
		targetDir = filepath.Dir(req.FilePath)
	} else if req.TargetDir != "" {
		targetDir = req.TargetDir
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return "", err
		}
	} else {
		if w.workspacePath == "" {
			return "", fmt.Errorf("no workspace open — open or create a workspace first")
		}
		targetDir = filepath.Join(w.workspacePath, "requests")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return "", err
		}
	}

	targetPath := filepath.Join(targetDir, slug+".yaml")

	// Conflict: target exists and is a different file from the one we loaded.
	// Use case-insensitive comparison to handle case-only renames on Windows/macOS.
	if !strings.EqualFold(targetPath, req.FilePath) {
		if _, err := os.Stat(targetPath); err == nil {
			return "", fmt.Errorf("a request named %q already exists in this folder — rename it first", name)
		}
	}

	domReq := requestFileIOToDomain(req)
	data, err := serialization.WriteRequest(domReq, "yaml")
	if err != nil {
		return "", err
	}
	if err := atomicWrite(targetPath, data); err != nil {
		return "", err
	}

	// Name changed → delete the old file after the new one is written.
	// Skip for case-only renames (same file, different case) to avoid self-deletion.
	if req.FilePath != "" && !strings.EqualFold(req.FilePath, targetPath) {
		_ = os.Remove(req.FilePath)
	}

	return targetPath, nil
}

// CreateFolder creates a new subdirectory for organising requests.
// If parentPath is empty, the folder is created under {workspace}/requests/.
func (w *WorkspaceService) CreateFolder(parentPath, name string) (string, error) {
	if parentPath == "" {
		if w.workspacePath == "" {
			return "", fmt.Errorf("no workspace open — open or create a workspace first")
		}
		parentPath = filepath.Join(w.workspacePath, "requests")
		if err := os.MkdirAll(parentPath, 0755); err != nil {
			return "", err
		}
	}
	if w.workspacePath != "" {
		rel, err := filepath.Rel(w.workspacePath, parentPath)
		if err != nil || outsideRel(rel) {
			return "", fmt.Errorf("path is outside the current workspace")
		}
	}
	dirName := sanitizeDirName(name)
	if dirName == "" {
		return "", fmt.Errorf("invalid folder name")
	}
	newPath := filepath.Join(parentPath, dirName)
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("a folder named %q already exists here", name)
	}
	if err := os.MkdirAll(newPath, 0755); err != nil {
		return "", fmt.Errorf("creating folder: %w", err)
	}
	return newPath, nil
}

// DeleteFolder removes an empty folder from the workspace.
// Hidden files (e.g. .DS_Store) are ignored when checking emptiness.
func (w *WorkspaceService) DeleteFolder(path string) error {
	if w.workspacePath == "" {
		return fmt.Errorf("open a workspace first")
	}
	rel, err := filepath.Rel(w.workspacePath, path)
	if err != nil || outsideRel(rel) {
		return fmt.Errorf("path is outside the current workspace")
	}
	if rel == "." {
		return fmt.Errorf("the workspace folder itself cannot be deleted here")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			return fmt.Errorf("folder is not empty")
		}
		// Hidden files (.DS_Store and the like) do not make a folder non-empty; a hidden folder
		// (.git, .clrsig-coverage) holds real content and is never removed along with it.
		if e.IsDir() {
			return fmt.Errorf("folder contains the hidden folder %s; remove it yourself first", e.Name())
		}
	}
	return os.RemoveAll(path)
}

// RenameFolder renames a workspace folder.
// Returns the new path.
func (w *WorkspaceService) RenameFolder(path, newName string) (string, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return "", fmt.Errorf("name cannot be empty")
	}
	if w.workspacePath != "" {
		rel, err := filepath.Rel(w.workspacePath, path)
		if err != nil || outsideRel(rel) {
			return "", fmt.Errorf("path is outside the current workspace")
		}
	}
	dirName := sanitizeDirName(newName)
	if dirName == "" {
		return "", fmt.Errorf("invalid folder name")
	}
	newPath := filepath.Join(filepath.Dir(path), dirName)
	if newPath == path {
		return path, nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("a folder named %q already exists here", newName)
	}
	if err := os.Rename(path, newPath); err != nil {
		return "", fmt.Errorf("renaming folder: %w", err)
	}
	return newPath, nil
}

// RenameRequest renames a request file, updating the name field inside the YAML.
// Returns the new file path.
func (w *WorkspaceService) RenameRequest(path, newName string) (string, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return "", fmt.Errorf("name cannot be empty")
	}
	if w.workspacePath != "" {
		rel, err := filepath.Rel(w.workspacePath, path)
		if err != nil || outsideRel(rel) {
			return "", fmt.Errorf("path is outside the current workspace")
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	req, err := serialization.ParseRequestFile(data, filepath.Base(path))
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}
	req.Title = newName

	slug := sanitizeDirName(newName)
	if slug == "" {
		slug = "request"
	}
	newPath := filepath.Join(filepath.Dir(path), slug+".yaml")
	if newPath != path {
		if _, err := os.Stat(newPath); err == nil {
			return "", fmt.Errorf("a request named %q already exists in this folder", newName)
		}
	}
	newData, err := serialization.WriteRequest(req, "yaml")
	if err != nil {
		return "", err
	}
	if err := atomicWrite(newPath, newData); err != nil {
		return "", err
	}
	if newPath != path {
		_ = os.Remove(path)
	}
	return newPath, nil
}

// MoveRequest moves a request file into a different folder within the workspace.
// Returns the new file path.
func (w *WorkspaceService) MoveRequest(srcPath, destDir string) (string, error) {
	if w.workspacePath != "" {
		for _, p := range []string{srcPath, destDir} {
			rel, err := filepath.Rel(w.workspacePath, p)
			if err != nil || outsideRel(rel) {
				return "", fmt.Errorf("path is outside the current workspace")
			}
		}
	}
	newPath := filepath.Join(destDir, filepath.Base(srcPath))
	if newPath == srcPath {
		return srcPath, nil // already in this folder
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("creating destination: %w", err)
	}
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("a file named %q already exists in the destination folder", filepath.Base(srcPath))
	}
	if err := os.Rename(srcPath, newPath); err != nil {
		return "", fmt.Errorf("moving file: %w", err)
	}
	return newPath, nil
}

// CopyRequest copies a request file into a different folder within the workspace.
// Returns the new file path, or an error.
func (w *WorkspaceService) CopyRequest(srcPath, destDir string) (string, error) {
	if w.workspacePath != "" {
		for _, p := range []string{srcPath, destDir} {
			rel, err := filepath.Rel(w.workspacePath, p)
			if err != nil || outsideRel(rel) {
				return "", fmt.Errorf("path is outside the current workspace")
			}
		}
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("creating destination: %w", err)
	}
	base := filepath.Base(srcPath)
	newPath := filepath.Join(destDir, base)
	if _, err := os.Stat(newPath); err == nil {
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		for i := 2; ; i++ {
			if i == 2 {
				newPath = filepath.Join(destDir, stem+"-copy"+ext)
			} else {
				newPath = filepath.Join(destDir, fmt.Sprintf("%s-copy-%d%s", stem, i, ext))
			}
			if _, err := os.Stat(newPath); err != nil {
				break
			}
		}
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("reading source: %w", err)
	}
	if err := atomicWrite(newPath, data); err != nil {
		return "", fmt.Errorf("writing copy: %w", err)
	}
	return newPath, nil
}

// DeleteRequest removes a request file from disk.
func (w *WorkspaceService) DeleteRequest(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	// Safety: only allow deletion of files within the workspace.
	if w.workspacePath != "" {
		rel, err := filepath.Rel(w.workspacePath, path)
		if err != nil || outsideRel(rel) {
			return fmt.Errorf("path is outside the current workspace")
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting %s: %w", path, err)
	}
	return nil
}

// OpenRequest opens a file picker and loads a request file.
func (w *WorkspaceService) OpenRequest() (*RequestFileIO, error) {
	defaultDir := w.workspacePath
	if defaultDir == "" {
		defaultDir, _ = os.UserHomeDir()
	}
	path, err := runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		Title:            "Open Request",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{DisplayName: "Request files (*.yaml, *.yml, *.json)", Pattern: "*.yaml;*.yml;*.json"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled
	}
	return w.LoadRequest(path)
}

func isDataFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}

func isRequestFile(path string) bool {
	// Quick heuristic: if it's a data file we treat it as a potential request.
	// A full parse happens only when the user loads it.
	return isDataFile(filepath.Base(path))
}

// peekMethod reads just the method field from a request file without full parsing.
func peekMethod(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var out struct {
		Method string `yaml:"method" json:"method"`
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		if err := json.Unmarshal(data, &out); err != nil {
			return ""
		}
	} else {
		if err := yaml.Unmarshal(data, &out); err != nil {
			return ""
		}
	}
	return strings.ToUpper(out.Method)
}

func buildFileTree(dir string) ([]FileNodeIO, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var nodes []FileNodeIO
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		if e.IsDir() {
			children, err := buildFileTree(path)
			if err != nil {
				continue
			}
			nodes = append(nodes, FileNodeIO{Name: name, Path: path, IsDir: true, Children: children})
		} else if isRequestFile(path) {
			nodes = append(nodes, FileNodeIO{Name: name, Path: path, IsDir: false, Method: peekMethod(path)})
		}
	}
	return nodes, nil
}

func domainToRequestFileIO(req *domain.Request) *RequestFileIO {
	out := &RequestFileIO{
		SchemaVersion: req.SchemaVersion,
		Title:         req.Title,
		Method:        string(req.Method),
		URL:           req.URL,
		Extensions:    string(req.Extensions),
	}
	for _, h := range req.Headers {
		out.Headers = append(out.Headers, HeaderIO{Name: h.Name, Value: h.Value, Extensions: string(h.Extensions)})
	}
	for _, p := range req.QueryParams {
		out.QueryParams = append(out.QueryParams, QueryParamIO{Name: p.Name, Value: p.Value, Extensions: string(p.Extensions)})
	}
	if req.Body != nil {
		out.Body = &BodyIO{Type: string(req.Body.Type), Content: req.Body.Content, Extensions: string(req.Body.Extensions)}
	}
	out.Auth = authDomainToIO(req.Auth)
	if req.Timeout > 0 {
		out.TimeoutMs = req.Timeout.Milliseconds()
	}
	out.NoFollowRedirects = req.NoFollowRedirects
	return out
}

// authDomainToIO maps a request's auth block for the UI; nil stays nil.
func authDomainToIO(a *domain.Auth) *AuthIO {
	if a == nil {
		return nil
	}
	af := &AuthIO{Type: string(a.Type), Extensions: string(a.Extensions)}
	if a.Bearer != nil {
		af.Token = a.Bearer.Token
	}
	if a.Basic != nil {
		af.Username = a.Basic.Username
		af.Password = a.Basic.Password
	}
	if a.APIKey != nil {
		af.Key = a.APIKey.Key
		af.Value = a.APIKey.Value
		af.Location = string(a.APIKey.Location)
	}
	return af
}

func requestFileIOToDomain(r RequestFileIO) *domain.Request {
	req := &domain.Request{
		Title:  r.Title,
		Method: domain.HTTPMethod(r.Method),
		URL:    r.URL,

		Extensions: domain.Extensions(r.Extensions),
	}
	for _, h := range r.Headers {
		req.Headers = append(req.Headers, domain.Header{Name: h.Name, Value: h.Value, Extensions: domain.Extensions(h.Extensions)})
	}
	for _, p := range r.QueryParams {
		req.QueryParams = append(req.QueryParams, domain.QueryParam{Name: p.Name, Value: p.Value, Extensions: domain.Extensions(p.Extensions)})
	}
	if r.Body != nil {
		req.Body = &domain.RequestBody{Type: domain.BodyType(r.Body.Type), Content: r.Body.Content, Extensions: domain.Extensions(r.Body.Extensions)}
	}
	if r.Auth != nil {
		req.Auth = inputAuthToDomain(r.Auth)
	}
	if r.TimeoutMs > 0 {
		req.Timeout = time.Duration(r.TimeoutMs) * time.Millisecond
	}
	req.NoFollowRedirects = r.NoFollowRedirects
	return req
}
