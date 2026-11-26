package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The last response of each request, kept between sessions.

// ---- Response cache ----

// ResponseCacheIO holds the last response for a request file, persisted to .clrsigcache/.
type ResponseCacheIO struct {
	CachedAt   string     `json:"cachedAt"`
	StatusCode int        `json:"statusCode"`
	Status     string     `json:"status"`
	Headers    []HeaderIO `json:"headers,omitempty"`
	Body       string     `json:"body"`
	DurationMs int64      `json:"durationMs"`
	Size       int64      `json:"size"`
	Timing     TimingIO   `json:"timing"`
}

func (w *WorkspaceService) cachePath(reqPath string) (string, error) {
	if w.workspacePath == "" {
		return "", fmt.Errorf("no workspace open")
	}
	rel, err := filepath.Rel(w.workspacePath, reqPath)
	if err != nil || outsideRel(rel) {
		return "", fmt.Errorf("path is outside the current workspace")
	}
	base := strings.TrimSuffix(strings.TrimSuffix(rel, ".yaml"), ".yml")
	return filepath.Join(w.workspacePath, ".clrsigcache", base+".json"), nil
}

// SaveResponseCache persists the last response for a request file.
func (w *WorkspaceService) SaveResponseCache(reqPath string, response ResponseCacheIO) error {
	path, err := w.cachePath(reqPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	response.CachedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

// ClearResponseCache deletes the cached response for a request file.
func (w *WorkspaceService) ClearResponseCache(reqPath string) error {
	path, err := w.cachePath(reqPath)
	if err != nil {
		return nil // no workspace — nothing to clear
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// LoadResponseCache returns the last cached response for a request file, or nil if none exists.
func (w *WorkspaceService) LoadResponseCache(reqPath string) (*ResponseCacheIO, error) {
	path, err := w.cachePath(reqPath)
	if err != nil {
		return nil, nil // no workspace — silently return nothing
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cache ResponseCacheIO
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}
