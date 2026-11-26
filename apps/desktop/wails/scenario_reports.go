package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fuvito/api-workbench/packages/core/coverage"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/reporting"
	"github.com/fuvito/api-workbench/packages/core/scenario"
)

// Run reports and accumulated coverage.

// GetAccumulatedCoverage returns the accumulated spec coverage for the given specFile.
// Returns nil (no error) when no data has been recorded yet.
func (s *ScenarioService) GetAccumulatedCoverage(specFile string) (*AccumulatedCoverageIO, error) {
	if s.workspace.workspacePath == "" || specFile == "" {
		return nil, nil
	}
	stored := loadStoredCoverage(s.workspace.workspacePath, specFile)
	if stored.TotalRuns == 0 {
		return nil, nil
	}
	specData, specExt, err := loadWorkspaceSpec(s.workspace.workspacePath, specFile)
	if err != nil {
		return nil, fmt.Errorf("loading spec: %w", err)
	}
	spec, err := openapi.ParseBytes(specData, specExt)
	if err != nil {
		return nil, fmt.Errorf("parsing spec: %w", err)
	}
	acc := buildAccumulatedCoverageIO(stored, spec)
	return &acc, nil
}

// RunReportIO is an exported run report, or the reason it could not be made.
type RunReportIO struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// ExportRunReport renders the latest results of the given scenarios as one report, with the same
// Go renderers the CLI uses (and the same redaction of secrets). format is html, junit, json,
// markdown or text. Scenarios that have not been run are left out.
func (s *ScenarioService) ExportRunReport(format string, paths []string) RunReportIO {
	s.mu.Lock()
	var results []*scenario.Result
	var total time.Duration
	for _, p := range paths {
		if r, ok := s.runs[p]; ok {
			results = append(results, r)
			total += r.Duration
		}
	}
	s.mu.Unlock()
	if len(results) == 0 {
		return RunReportIO{Error: "there are no results to export; run the scenarios first"}
	}

	const title = "Run results"
	switch format {
	case "html":
		b, err := reporting.HTMLMulti(title, results, total)
		if err != nil {
			return RunReportIO{Error: fmt.Sprintf("generating the report: %v", err)}
		}
		return RunReportIO{Content: string(b)}
	case "junit":
		b, err := reporting.JUnitMulti(title, results)
		if err != nil {
			return RunReportIO{Error: fmt.Sprintf("generating the report: %v", err)}
		}
		return RunReportIO{Content: string(b)}
	case "json":
		b, err := reporting.JSONMulti(title, results, nil, total)
		if err != nil {
			return RunReportIO{Error: fmt.Sprintf("generating the report: %v", err)}
		}
		return RunReportIO{Content: string(b)}
	case "markdown":
		return RunReportIO{Content: string(reporting.MarkdownMulti(title, results, total))}
	case "text":
		return RunReportIO{Content: reporting.TextMulti(title, results, nil, total)}
	}
	return RunReportIO{Error: fmt.Sprintf("unknown report format %q (use html, junit, json, markdown or text)", format)}
}

// CoverageReportIO is an exported Contracts report, or the reason it could not be made.
type CoverageReportIO struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// ExportCoverageReport renders the accumulated coverage of specFile as csv, markdown or html with
// the same Go renderers the CLI's coverage command uses.
func (s *ScenarioService) ExportCoverageReport(specFile, format string) CoverageReportIO {
	if s.workspace.workspacePath == "" || specFile == "" {
		return CoverageReportIO{Error: "open a workspace and choose a spec first"}
	}
	stored := loadStoredCoverage(s.workspace.workspacePath, specFile)
	if stored.TotalRuns == 0 {
		return CoverageReportIO{Error: "no coverage has been recorded for this spec yet"}
	}
	specData, specExt, err := loadWorkspaceSpec(s.workspace.workspacePath, specFile)
	if err != nil {
		return CoverageReportIO{Error: fmt.Sprintf("loading spec: %v", err)}
	}
	spec, err := openapi.ParseBytes(specData, specExt)
	if err != nil {
		return CoverageReportIO{Error: fmt.Sprintf("parsing spec: %v", err)}
	}
	r, q := computeCoverage(stored, spec)
	now := time.Now()
	switch format {
	case "csv":
		return CoverageReportIO{Content: string(reporting.ContractsCSV(r, q))}
	case "markdown":
		return CoverageReportIO{Content: string(reporting.ContractsMarkdown(specFile, r, q, now))}
	case "html":
		b, err := reporting.ContractsHTML(specFile, r, q, now)
		if err != nil {
			return CoverageReportIO{Error: fmt.Sprintf("generating the report: %v", err)}
		}
		return CoverageReportIO{Content: string(b)}
	}
	return CoverageReportIO{Error: fmt.Sprintf("unknown report format %q (use csv, markdown or html)", format)}
}

// ResetAccumulatedCoverage deletes the stored coverage data for the given specFile.
func (s *ScenarioService) ResetAccumulatedCoverage(specFile string) error {
	if s.workspace.workspacePath == "" || specFile == "" {
		return nil
	}
	return coverage.Reset(s.workspace.workspacePath, specFile)
}

// ListAccumulatedCoverage returns all stored accumulated coverage entries for the workspace,
// sorted by most recently run first. Uses cached TotalOps so no spec files are reloaded.
func (s *ScenarioService) ListAccumulatedCoverage() []AccumulatedCoverageIO {
	wsPath := s.workspace.workspacePath
	if wsPath == "" {
		return nil
	}
	reports := coverage.ListAll(wsPath)
	result := make([]AccumulatedCoverageIO, len(reports))
	for i, r := range reports {
		result[i] = reportToIO(r)
	}
	return result
}

// loadSpecBytes fetches the OpenAPI spec at specRef (HTTP/HTTPS URL or file path).
// scenarioDir is used to resolve relative paths; it is ignored for URLs.
// Returns the raw bytes and a format hint ("yaml" or "json").
func loadSpecBytes(specRef, scenarioDir string) (data []byte, format string, err error) {
	if strings.HasPrefix(specRef, "http://") || strings.HasPrefix(specRef, "https://") {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, specRef, nil)
		if reqErr != nil {
			return nil, "", reqErr
		}
		resp, httpErr := http.DefaultClient.Do(req)
		if httpErr != nil {
			return nil, "", httpErr
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("HTTP %d fetching spec", resp.StatusCode)
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, "", readErr
		}
		// Detect format from URL path; default to yaml.
		format = "yaml"
		if strings.HasSuffix(strings.ToLower(specRef), ".json") {
			format = "json"
		}
		return body, format, nil
	}

	// File path — resolve relative paths against the scenario's directory.
	if !filepath.IsAbs(specRef) {
		specRef = filepath.Join(scenarioDir, specRef)
	}
	body, readErr := os.ReadFile(specRef)
	if readErr != nil {
		return nil, "", readErr
	}
	format = strings.TrimPrefix(strings.ToLower(filepath.Ext(specRef)), ".")
	if format == "" {
		format = "yaml"
	}
	return body, format, nil
}
