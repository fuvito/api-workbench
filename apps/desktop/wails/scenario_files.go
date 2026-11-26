package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/fuvito/api-workbench/packages/core/scenario"
	"github.com/fuvito/api-workbench/packages/core/serialization"
)

// Reading, writing and arranging scenario files and their folders.

func (s *ScenarioService) resultCachePath(scenarioPath string) (string, error) {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return "", fmt.Errorf("no workspace open")
	}
	rel, err := filepath.Rel(wsPath, scenarioPath)
	if err != nil || outsideRel(rel) {
		return "", fmt.Errorf("path is outside the current workspace")
	}
	base := strings.TrimSuffix(strings.TrimSuffix(rel, ".yaml"), ".yml")
	return filepath.Join(wsPath, ".clrsigcache", base+".result.json"), nil
}

// SaveScenarioResult persists the last run result for a scenario file.
// Returns "" on success or an error message.
func (s *ScenarioService) SaveScenarioResult(scenarioPath string, result ScenarioResultIO) string {
	path, err := s.resultCachePath(scenarioPath)
	if err != nil {
		return err.Error()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err.Error()
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err.Error()
	}
	if err := atomicWrite(path, data); err != nil {
		return err.Error()
	}
	return ""
}

// LoadScenarioResult returns the last cached run result for a scenario file, or nil if none exists.
func (s *ScenarioService) LoadScenarioResult(scenarioPath string) *ScenarioResultIO {
	path, err := s.resultCachePath(scenarioPath)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var result ScenarioResultIO
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}

// ListScenarios returns all scenario files under {workspace}/scenarios/.
func (s *ScenarioService) ListScenarios() []ScenarioInfoIO {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return nil
	}
	scenDir := filepath.Join(wsPath, "scenarios")
	var out []ScenarioInfoIO
	_ = filepath.WalkDir(scenDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p == scenDir {
				return nil // skip root itself
			}
			// Emit a folder sentinel so the UI can render empty directories.
			rel, _ := filepath.Rel(scenDir, p)
			out = append(out, ScenarioInfoIO{IsFolder: true, Folder: filepath.ToSlash(rel), Path: p})
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			return nil
		}
		name := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		rel, _ := filepath.Rel(scenDir, filepath.Dir(p))
		folder := ""
		if rel != "." {
			folder = filepath.ToSlash(rel)
		}
		info := ScenarioInfoIO{Title: name, Path: p, Folder: folder}
		if data, err := os.ReadFile(p); err == nil {
			if sc, err := serialization.ParseScenarioFile(data, d.Name()); err == nil {
				info.StepCount = len(sc.Steps)
			}
		}
		out = append(out, info)
		return nil
	})
	return out
}

// LoadScenario parses a scenario file and returns its structure for the UI.
func (s *ScenarioService) LoadScenario(path string) ScenarioDetailIO {
	data, err := os.ReadFile(path)
	if err != nil {
		return ScenarioDetailIO{Path: path, Error: err.Error()}
	}
	sc, err := serialization.ParseScenarioFile(data, filepath.Base(path))
	if err != nil {
		return ScenarioDetailIO{Path: path, Error: err.Error()}
	}
	detail := ScenarioDetailIO{
		Path:        path,
		Title:       sc.Title,
		Description: sc.Description,
		SpecFile:    sc.SpecFile,
		Extensions:  string(sc.Extensions),
	}
	for _, v := range sc.Variables {
		detail.Variables = append(detail.Variables, VariableIO{Name: v.Name, Value: v.Value, Secret: v.Secret, Extensions: string(v.Extensions)})
	}
	for _, step := range sc.Steps {
		sd := StepDetailIO{
			Label:       step.Label,
			Notes:       step.Notes,
			Disabled:    step.Disabled,
			RequestRef:  step.RequestRef,
			FailPolicy:  string(step.FailPolicy),
			TimeoutMs:   step.TimeoutMs,
			Extensions:  string(step.Extensions),
			Headers:     []KVPairIO{},
			QueryParams: []KVPairIO{},
			Assertions:  []AssertionDetailIO{},
			Extractions: []ExtractionDetailIO{},
		}
		if step.Retry != nil && step.Retry.MaxAttempts > 0 {
			sd.RetryMaxAttempts = step.Retry.MaxAttempts
			sd.RetryDelayMs = int(step.Retry.Delay.Milliseconds())
			sd.RetryExtensions = string(step.Retry.Extensions)
		}
		if step.Request != nil {
			sd.Method = string(step.Request.Method)
			sd.URL = step.Request.URL
			sd.NoFollowRedirects = step.Request.NoFollowRedirects
			sd.Auth = authDomainToIO(step.Request.Auth)
			sd.RequestTitle = step.Request.Title
			sd.RequestExtensions = string(step.Request.Extensions)
			if step.Request.Timeout > 0 {
				sd.RequestTimeout = step.Request.Timeout.String()
			}
			for _, h := range step.Request.Headers {
				sd.Headers = append(sd.Headers, KVPairIO{Name: h.Name, Value: h.Value, Extensions: string(h.Extensions)})
			}
			for _, q := range step.Request.QueryParams {
				sd.QueryParams = append(sd.QueryParams, KVPairIO{Name: q.Name, Value: q.Value, Extensions: string(q.Extensions)})
			}
			if step.Request.Body != nil && step.Request.Body.Type != "" {
				sd.BodyType = string(step.Request.Body.Type)
				sd.BodyContent = step.Request.Body.Content
				sd.BodyExtensions = string(step.Request.Body.Extensions)
			}
		}
		for _, a := range step.Assertions {
			sd.Assertions = append(sd.Assertions, AssertionDetailIO{
				Type:     string(a.Type),
				Name:     a.Name,
				Path:     a.Path,
				Operator: string(a.Operator),
				Expected: a.Expected,
				Schema:   a.Schema,

				Extensions: string(a.Extensions),
			})
		}
		for _, e := range step.Extractions {
			sd.Extractions = append(sd.Extractions, ExtractionDetailIO{
				Name:   e.Name,
				Source: string(e.Source),
				Path:   e.Path,

				Extensions: string(e.Extensions),
			})
		}
		detail.Steps = append(detail.Steps, sd)
	}
	return detail
}

// LoadScenarioRaw returns the raw file content of a scenario file as a string.
func (s *ScenarioService) LoadScenarioRaw(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// SaveScenarioRaw atomically writes content back to a scenario file.
// Returns an error message on failure, or "" on success.
func (s *ScenarioService) SaveScenarioRaw(path string, content string) string {
	if err := atomicWrite(path, []byte(content)); err != nil {
		return err.Error()
	}
	return ""
}

// ValidateScenarioYAML parses and validates YAML content as a scenario file.
// Returns "" on success, or a human-readable error/issue list on failure.
func (s *ScenarioService) ValidateScenarioYAML(content string) string {
	sc, err := serialization.ParseScenario([]byte(content), "yaml")
	if err != nil {
		return "YAML parse error: " + err.Error()
	}
	issues := scenario.Validate(sc, "")
	if len(issues) == 0 {
		return ""
	}
	msgs := make([]string, len(issues))
	for i, iss := range issues {
		msgs[i] = iss.String()
	}
	return strings.Join(msgs, "\n")
}

// CreateScenarioResultIO is the return type for CreateScenario.
type CreateScenarioResultIO struct {
	Path  string `json:"path"`
	Error string `json:"error,omitempty"`
}

// CreateScenario creates a starter scenario YAML file. folderPath is relative to
// {workspace}/scenarios/ (e.g. "auth" or ""). Returns the new file path, or an error.
func (s *ScenarioService) CreateScenario(name, folderPath string) CreateScenarioResultIO {
	if name == "" {
		return CreateScenarioResultIO{Error: "name is required"}
	}
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return CreateScenarioResultIO{Error: "no workspace open"}
	}

	scenDir := filepath.Join(wsPath, "scenarios")
	targetDir := scenDir
	if folderPath != "" {
		clean := filepath.Clean(filepath.FromSlash(folderPath))
		if strings.HasPrefix(clean, "..") {
			return CreateScenarioResultIO{Error: "invalid folder path"}
		}
		targetDir = filepath.Join(scenDir, clean)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return CreateScenarioResultIO{Error: err.Error()}
	}

	slug := toSlug(name)
	if slug == "" {
		slug = "scenario"
	}
	filePath := filepath.Join(targetDir, slug+".yaml")

	// Avoid overwriting an existing file by appending a suffix.
	if _, err := os.Stat(filePath); err == nil {
		for i := 2; ; i++ {
			candidate := filepath.Join(targetDir, fmt.Sprintf("%s-%d.yaml", slug, i))
			if _, err := os.Stat(candidate); err != nil {
				filePath = candidate
				break
			}
		}
	}

	// A JSON string is valid YAML, so titles containing ':' or '#' stay parseable.
	quotedName, _ := json.Marshal(name)
	starter := fmt.Sprintf(`schemaVersion: "1"
title: %s
description: ""
steps:
  - label: Step 1
    request:
      method: GET
      url: https://api.example.com
    assertions:
      - type: status
        operator: eq
        expected: 200
`, quotedName)

	if err := atomicWrite(filePath, []byte(starter)); err != nil {
		return CreateScenarioResultIO{Error: err.Error()}
	}
	return CreateScenarioResultIO{Path: filePath}
}

func toSlug(s string) string {
	var out strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			prev = r
		} else if prev != '-' {
			out.WriteRune('-')
			prev = '-'
		}
	}
	return strings.Trim(out.String(), "-")
}

// DuplicateScenario copies a scenario file into the same directory with a "-copy" suffix.
// Returns the new file path on success, or an error message in the Error field.
func (s *ScenarioService) DuplicateScenario(path string) CreateScenarioResultIO {
	data, err := os.ReadFile(path)
	if err != nil {
		return CreateScenarioResultIO{Error: err.Error()}
	}

	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	newBase := base + "-copy"
	newPath := filepath.Join(dir, newBase+".yaml")
	for i := 2; ; i++ {
		if _, err := os.Stat(newPath); err != nil {
			break
		}
		newPath = filepath.Join(dir, fmt.Sprintf("%s-%d.yaml", newBase, i))
	}

	if err := atomicWrite(newPath, data); err != nil {
		return CreateScenarioResultIO{Error: err.Error()}
	}
	return CreateScenarioResultIO{Path: newPath}
}

// DeleteScenario removes a scenario file. Returns "" on success or an error message.
func (s *ScenarioService) DeleteScenario(path string) string {
	if err := os.Remove(path); err != nil {
		return err.Error()
	}
	return ""
}

// RenameScenario renames a scenario file to {dir}/{slug(newName)}.yaml.
// Returns the new path on success, or an error message prefixed with "error:".
func (s *ScenarioService) RenameScenario(path, newName string) string {
	if newName == "" {
		return "error: name is required"
	}
	slug := toSlug(newName)
	if slug == "" {
		slug = "scenario"
	}
	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, slug+".yaml")
	if newPath != path {
		if _, err := os.Stat(newPath); err == nil {
			return "error: a scenario with that name already exists"
		}
		if err := os.Rename(path, newPath); err != nil {
			return "error: " + err.Error()
		}
	}
	return newPath
}

// MoveScenario moves a scenario file into a different folder under {workspace}/scenarios/.
// destFolderRelPath is relative to scenarios/ (use "" for the root).
// Returns the new absolute path on success, or an error.
func (s *ScenarioService) MoveScenario(srcPath, destFolderRelPath string) (string, error) {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return "", fmt.Errorf("no workspace open")
	}
	scenDir := filepath.Join(wsPath, "scenarios")
	rel, err := filepath.Rel(scenDir, srcPath)
	if err != nil || outsideRel(rel) {
		return "", fmt.Errorf("source is outside the scenarios directory")
	}
	var destDir string
	if destFolderRelPath == "" {
		destDir = scenDir
	} else {
		clean := filepath.Clean(filepath.FromSlash(destFolderRelPath))
		if strings.HasPrefix(clean, "..") {
			return "", fmt.Errorf("invalid destination folder path")
		}
		destDir = filepath.Join(scenDir, clean)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("creating destination: %w", err)
	}
	newPath := filepath.Join(destDir, filepath.Base(srcPath))
	if newPath == srcPath {
		return srcPath, nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("a file named %q already exists in the destination folder", filepath.Base(srcPath))
	}
	if err := os.Rename(srcPath, newPath); err != nil {
		return "", fmt.Errorf("moving file: %w", err)
	}
	return newPath, nil
}

// CopyScenario copies a scenario file into a different folder under {workspace}/scenarios/.
// destFolderRelPath is relative to scenarios/ (use "" for the root).
// Returns the new absolute path on success, or an error.
func (s *ScenarioService) CopyScenario(srcPath, destFolderRelPath string) (string, error) {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return "", fmt.Errorf("no workspace open")
	}
	scenDir := filepath.Join(wsPath, "scenarios")
	rel, err := filepath.Rel(scenDir, srcPath)
	if err != nil || outsideRel(rel) {
		return "", fmt.Errorf("source is outside the scenarios directory")
	}
	var destDir string
	if destFolderRelPath == "" {
		destDir = scenDir
	} else {
		clean := filepath.Clean(filepath.FromSlash(destFolderRelPath))
		if strings.HasPrefix(clean, "..") {
			return "", fmt.Errorf("invalid destination folder path")
		}
		destDir = filepath.Join(scenDir, clean)
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

// CreateScenarioFolder creates a subfolder under {workspace}/scenarios/.
// folderPath is relative to scenarios/ (e.g. "auth" or "auth/subsection").
// Returns "" on success or an error message.
func (s *ScenarioService) CreateScenarioFolder(folderPath string) string {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return "no workspace open"
	}
	if folderPath == "" {
		return "folder name is required"
	}
	clean := filepath.Clean(filepath.FromSlash(folderPath))
	if strings.HasPrefix(clean, "..") {
		return "invalid folder path"
	}
	dir := filepath.Join(wsPath, "scenarios", clean)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err.Error()
	}
	return ""
}

// RenameScenarioFolder renames the last segment of a folder.
// folderPath is the current relative path from scenarios/; newName is just the new last segment.
// Returns the new relative path on success, or "error: ..." on failure.
func (s *ScenarioService) RenameScenarioFolder(folderPath, newName string) string {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return "error: no workspace open"
	}
	if newName == "" {
		return "error: name is required"
	}
	clean := filepath.Clean(filepath.FromSlash(folderPath))
	if strings.HasPrefix(clean, "..") || clean == "." {
		return "error: invalid folder path"
	}
	slug := toSlug(newName)
	if slug == "" {
		return "error: invalid folder name"
	}
	scenDir := filepath.Join(wsPath, "scenarios")
	oldAbs := filepath.Join(scenDir, clean)
	newAbs := filepath.Join(filepath.Dir(oldAbs), slug)
	if oldAbs == newAbs {
		return folderPath
	}
	if _, err := os.Stat(newAbs); err == nil {
		return "error: a folder with that name already exists"
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return "error: " + err.Error()
	}
	rel, _ := filepath.Rel(scenDir, newAbs)
	return filepath.ToSlash(rel)
}

// DeleteScenarioFolder removes a folder and all its contents.
// folderPath is relative to {workspace}/scenarios/.
// Returns "" on success or an error message.
func (s *ScenarioService) DeleteScenarioFolder(folderPath string) string {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return "no workspace open"
	}
	clean := filepath.Clean(filepath.FromSlash(folderPath))
	if strings.HasPrefix(clean, "..") || clean == "." {
		return "invalid folder path"
	}
	dir := filepath.Join(wsPath, "scenarios", clean)
	if err := os.RemoveAll(dir); err != nil {
		return err.Error()
	}
	return ""
}
