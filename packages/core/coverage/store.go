// Package coverage provides workspace-scoped accumulative spec coverage tracking.
// Coverage data is persisted in {workspace}/.clrsig-coverage/ as JSON files,
// one per spec file or URL. Both the desktop app and CLI write to the same store.
package coverage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OperationResult records the accumulated run outcomes for a single spec operation.
type OperationResult struct {
	LastPassed  bool      `json:"lastPassed"`
	LastRunAt   time.Time `json:"lastRunAt"`
	StatusCodes []int     `json:"statusCodes,omitempty"` // all unique status codes observed across runs
	HadAuth     bool      `json:"hadAuth,omitempty"`     // ever exercised with auth credentials
	// LastDurationMs is the response time of the last run that got a response; SlowestDurationMs is
	// the slowest response time ever recorded for the operation. Both are absent in data written
	// before they existed.
	LastDurationMs    int64 `json:"lastDurationMs,omitempty"`
	SlowestDurationMs int64 `json:"slowestDurationMs,omitempty"`
}

// MergeStatusCodes merges two sorted, deduplicated status-code slices.
func MergeStatusCodes(a, b []int) []int {
	seen := make(map[int]struct{}, len(a)+len(b))
	for _, c := range a {
		seen[c] = struct{}{}
	}
	for _, c := range b {
		seen[c] = struct{}{}
	}
	out := make([]int, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

// StoredCoverage is the on-disk format for a single spec's accumulated data.
type StoredCoverage struct {
	SpecFile         string                     `json:"specFile"`
	Exercised        []string                   `json:"exercised"`
	TotalOps         int                        `json:"totalOps"` // cached so list views don't reload the spec
	TotalRuns        int                        `json:"totalRuns"`
	PassedRuns       int                        `json:"passedRuns"`
	FailedRuns       int                        `json:"failedRuns"`
	LastRunAt        time.Time                  `json:"lastRunAt"`
	OperationResults map[string]OperationResult `json:"operationResults,omitempty"`
}

// Report is the computed accumulated coverage view for a single spec.
type Report struct {
	SpecFile         string
	Covered          int
	Total            int
	Percent          float64
	TotalRuns        int
	PassedRuns       int
	FailedRuns       int
	LastRunAt        time.Time
	Exercised        []string
	Missed           []string // nil when built from Summary(); populated by BuildReport
	OperationResults map[string]OperationResult
}

// DirPath returns the coverage directory for the given workspace.
func DirPath(workspacePath string) string {
	return filepath.Join(workspacePath, ".clrsig-coverage")
}

// Load reads the stored coverage of the spec ref names (see Identify for what bases are).
// With no data it returns an empty entry that already carries the name to store it under.
func Load(workspacePath, ref string, bases ...string) StoredCoverage {
	_, cov, _ := find(workspacePath, ref, bases)
	return cov
}

// Save atomically writes cov to the workspace coverage store.
func Save(workspacePath string, cov StoredCoverage) error {
	if workspacePath == "" {
		return nil
	}
	cov.SpecFile = Identify(workspacePath, cov.SpecFile)
	return write(workspacePath, filepath.Join(DirPath(workspacePath), Key(cov.SpecFile)), cov)
}

func write(workspacePath, dst string, cov StoredCoverage) error {
	if err := os.MkdirAll(DirPath(workspacePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cov, "", "  ")
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Has reports whether coverage data is stored for the spec ref names.
func Has(workspacePath, ref string, bases ...string) bool {
	_, _, ok := find(workspacePath, ref, bases)
	return ok
}

// Reset deletes the stored coverage of the spec ref names.
func Reset(workspacePath, ref string, bases ...string) error {
	path, _, ok := find(workspacePath, ref, bases)
	if !ok {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// MergeExercised merges two slices of "METHOD /path" strings into a sorted, deduplicated result.
func MergeExercised(existing, newOps []string) []string {
	seen := make(map[string]bool, len(existing)+len(newOps))
	for _, op := range existing {
		seen[op] = true
	}
	for _, op := range newOps {
		seen[op] = true
	}
	merged := make([]string, 0, len(seen))
	for op := range seen {
		merged = append(merged, op)
	}
	sort.Strings(merged)
	return merged
}

// Summary builds a Report from cached data only — no spec file is loaded.
// The Missed field is nil; use BuildReport when you have the full op list.
func (sc StoredCoverage) Summary() Report {
	covered := len(sc.Exercised)
	total := sc.TotalOps
	if covered > total {
		covered = total
	}
	var pct float64
	if total > 0 {
		pct = float64(covered) / float64(total) * 100
	}
	ex := sc.Exercised
	if ex == nil {
		ex = []string{}
	}
	return Report{
		SpecFile:         sc.SpecFile,
		Covered:          covered,
		Total:            total,
		Percent:          pct,
		TotalRuns:        sc.TotalRuns,
		PassedRuns:       sc.PassedRuns,
		FailedRuns:       sc.FailedRuns,
		LastRunAt:        sc.LastRunAt,
		Exercised:        ex,
		OperationResults: sc.OperationResults,
	}
}

// BuildReport computes the full report including Missed operations.
// allOps is every operation declared in the spec, expressed as "METHOD /path" strings
// (e.g. from spec.AllOperations() converted via op.String()).
func BuildReport(stored StoredCoverage, allOps []string) Report {
	exercisedSet := make(map[string]bool, len(stored.Exercised))
	for _, op := range stored.Exercised {
		exercisedSet[op] = true
	}
	var missed []string
	for _, op := range allOps {
		if !exercisedSet[op] {
			missed = append(missed, op)
		}
	}
	total := len(allOps)
	covered := total - len(missed)
	var pct float64
	if total > 0 {
		pct = float64(covered) / float64(total) * 100
	}
	ex := stored.Exercised
	if ex == nil {
		ex = []string{}
	}
	return Report{
		SpecFile:         stored.SpecFile,
		Covered:          covered,
		Total:            total,
		Percent:          pct,
		TotalRuns:        stored.TotalRuns,
		PassedRuns:       stored.PassedRuns,
		FailedRuns:       stored.FailedRuns,
		LastRunAt:        stored.LastRunAt,
		Exercised:        ex,
		Missed:           missed,
		OperationResults: stored.OperationResults,
	}
}

// ListAll scans the coverage directory in workspacePath and returns all entries
// as summary Reports, sorted by most recently run first.
func ListAll(workspacePath string) []Report {
	upgrade(workspacePath)
	entries, err := os.ReadDir(DirPath(workspacePath))
	if err != nil {
		return nil
	}
	var result []Report
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(DirPath(workspacePath), e.Name()))
		if err != nil {
			continue
		}
		var sc StoredCoverage
		if err := json.Unmarshal(data, &sc); err != nil {
			continue
		}
		if sc.TotalRuns == 0 {
			continue
		}
		result = append(result, sc.Summary())
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastRunAt.After(result[j].LastRunAt)
	})
	return result
}

// MergeDuration returns the last response time to store for an operation: the one from this run
// when a response arrived, otherwise the one already stored.
func MergeDuration(prev OperationResult, thisRunLastMs int64, thisRunSlowestMs int64) int64 {
	if thisRunLastMs > 0 || thisRunSlowestMs > 0 {
		return thisRunLastMs
	}
	return prev.LastDurationMs
}
