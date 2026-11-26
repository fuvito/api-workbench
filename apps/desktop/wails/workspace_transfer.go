package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fuvito/api-workbench/packages/core/serialization"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Saving and picking files, and exporting and importing a whole workspace.

// SaveContentToFile opens a system save dialog and writes content to the chosen path.
// Returns the saved path, or an empty string if the user cancelled.
func (w *WorkspaceService) SaveContentToFile(content, defaultName string) (string, error) {
	path, err := runtime.SaveFileDialog(w.ctx, runtime.SaveDialogOptions{
		Title:           "Save File",
		DefaultFilename: defaultName,
	})
	if err != nil || path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}
	return path, nil
}

// PickFile opens a native file dialog and returns the selected path.
// Returns an empty string if the user cancels.
func (w *WorkspaceService) PickFile(title string) string {
	path, err := runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
	if err != nil {
		return ""
	}
	return path
}

// ---- Workspace export / import ----

type ScanImportResultIO struct {
	ZipPath    string   `json:"zipPath"`
	TotalFiles int      `json:"totalFiles"`
	Conflicts  []string `json:"conflicts"`
	Error      string   `json:"error"`
}

type ApplyImportResultIO struct {
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
	Renamed  int    `json:"renamed"`
	Error    string `json:"error"`
}

// WorkspaceHasSecrets returns true if any environment file in the workspace
// contains at least one variable marked secret: true with a non-empty value.
func (w *WorkspaceService) WorkspaceHasSecrets() bool {
	if w.workspacePath == "" {
		return false
	}
	envDir := filepath.Join(w.workspacePath, "environments")
	entries, err := os.ReadDir(envDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(envDir, name))
		if err != nil {
			continue
		}
		env, err := serialization.ParseEnvironmentFile(data, name)
		if err != nil {
			continue
		}
		for _, v := range env.Variables {
			if v.Secret && v.Value != "" {
				return true
			}
		}
	}
	return false
}

// ExportWorkspace packs requests/, scenarios/, and environments/ into a ZIP
// and saves it via a file dialog. Returns "" on success or an error message.
func (w *WorkspaceService) ExportWorkspace() string {
	if w.workspacePath == "" {
		return "no workspace open"
	}
	wsName := filepath.Base(w.workspacePath)
	defaultName := wsName + "-" + time.Now().Format("2006-01-02") + ".zip"
	savePath, err := runtime.SaveFileDialog(w.ctx, runtime.SaveDialogOptions{
		Title:           "Export Workspace",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP archive (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil || savePath == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(savePath), ".zip") {
		savePath += ".zip"
	}
	if err := createWorkspaceZip(w.workspacePath, savePath); err != nil {
		return err.Error()
	}
	return ""
}

// ScanWorkspaceImport opens a file picker for a ZIP, reads its manifest, and
// returns conflict information without extracting anything.
// A ZipPath of "" means the user cancelled.
func (w *WorkspaceService) ScanWorkspaceImport() ScanImportResultIO {
	if w.workspacePath == "" {
		return ScanImportResultIO{Error: "no workspace open"}
	}
	path, err := runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		Title: "Import Workspace ZIP",
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP archive (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil || path == "" {
		return ScanImportResultIO{}
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		return ScanImportResultIO{Error: "invalid ZIP: " + err.Error()}
	}
	defer r.Close()
	var conflicts []string
	total := 0
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		total++
		dest := filepath.Join(w.workspacePath, filepath.FromSlash(f.Name))
		if _, err := os.Stat(dest); err == nil {
			conflicts = append(conflicts, f.Name)
		}
	}
	return ScanImportResultIO{ZipPath: path, TotalFiles: total, Conflicts: conflicts}
}

// ApplyWorkspaceImport extracts zipPath into the current workspace.
// strategy: "skip" | "overwrite" | "rename"
func (w *WorkspaceService) ApplyWorkspaceImport(zipPath, strategy string) ApplyImportResultIO {
	if w.workspacePath == "" {
		return ApplyImportResultIO{Error: "no workspace open"}
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ApplyImportResultIO{Error: "invalid ZIP: " + err.Error()}
	}
	defer r.Close()
	result := ApplyImportResultIO{}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		dest := filepath.Join(w.workspacePath, filepath.FromSlash(f.Name))
		if rel, err := filepath.Rel(w.workspacePath, dest); err != nil || outsideRel(rel) {
			continue
		}
		_, existErr := os.Stat(dest)
		exists := existErr == nil
		if exists {
			switch strategy {
			case "skip":
				result.Skipped++
				continue
			case "rename":
				dest = findNonConflictingPath(dest)
				result.Renamed++
			default: // "overwrite" — fall through
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			result.Error = err.Error()
			return result
		}
		rc, err := f.Open()
		if err != nil {
			result.Error = err.Error()
			return result
		}
		data, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			result.Error = readErr.Error()
			return result
		}
		if err := atomicWrite(dest, data); err != nil {
			result.Error = err.Error()
			return result
		}
		result.Imported++
	}
	return result
}

// createWorkspaceZip writes requests/, scenarios/, and environments/ into a ZIP
// at destPath using an atomic temp-file + rename.
func createWorkspaceZip(workspacePath, destPath string) error {
	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating zip: %w", err)
	}
	success := false
	defer func() {
		if !success {
			os.Remove(tmp)
		}
	}()
	zw := zip.NewWriter(f)
	for _, dir := range []string{"requests", "scenarios", "environments"} {
		dirPath := filepath.Join(workspacePath, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}
		if err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || strings.HasPrefix(info.Name(), ".") {
				return nil
			}
			rel, err := filepath.Rel(workspacePath, path)
			if err != nil {
				return nil
			}
			w, err := zw.Create(filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		}); err != nil {
			f.Close()
			return fmt.Errorf("walking %s: %w", dir, err)
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return fmt.Errorf("closing zip: %w", err)
	}
	f.Close()
	success = true
	return os.Rename(tmp, destPath)
}

// findNonConflictingPath appends -2, -3, … to the stem until the path is free.
func findNonConflictingPath(path string) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}
