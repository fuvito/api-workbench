package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fuvito/api-workbench/packages/core/conformance"
	"github.com/fuvito/api-workbench/packages/core/history"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/redact"
	"github.com/fuvito/api-workbench/packages/core/reporting"
	"github.com/fuvito/api-workbench/packages/core/scenario"
	"github.com/fuvito/api-workbench/packages/core/serialization"
	"github.com/fuvito/api-workbench/packages/core/variables"
)

// Running scenarios, and what is remembered of the runs.

// FolderRunItemIO is a single request result from a folder run.
type FolderRunItemIO struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Method     string `json:"method"`
	StatusCode int    `json:"statusCode"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
	Passed     bool   `json:"passed"` // true when status is 2xx and no network error
}

// RunFolder executes every request file directly under folderPath (non-recursive)
// using the active workspace environment. Results are returned in directory order.
// CancelFolderRun stops it: the request in flight is abandoned (not reported) and the results so
// far are returned.
func (s *ScenarioService) RunFolder(folderPath string) []FolderRunItemIO {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return []FolderRunItemIO{{Error: err.Error()}}
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.cancelFolder = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.cancelFolder = nil
		s.mu.Unlock()
	}()

	env := s.workspace.ActiveEnvironment()
	exec := buildExecutor(s.settings)

	var results []FolderRunItemIO
	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		if e.IsDir() || !isDataFile(e.Name()) {
			continue
		}
		path := filepath.Join(folderPath, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			results = append(results, FolderRunItemIO{Name: e.Name(), Path: path, Error: err.Error()})
			continue
		}
		req, err := serialization.ParseRequestFile(data, e.Name())
		if err != nil {
			results = append(results, FolderRunItemIO{Name: e.Name(), Path: path, Error: "parse: " + err.Error()})
			continue
		}

		name := req.Title
		if name == "" {
			name = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		}

		// Apply variable interpolation with the active environment.
		resolver := variables.NewResolver(env)
		resolved := resolver.InterpolateRequest(req)

		if err := variables.NotSendable(resolver, env, resolved, howToGiveAValue); err != nil {
			results = append(results, FolderRunItemIO{Name: name, Path: path, Method: string(req.Method), Error: err.Error()})
			continue
		}

		resp, execErr := exec.Execute(ctx, resolved)
		if ctx.Err() != nil {
			break // cancelled while this request was in flight: it is not a result
		}
		if execErr != nil {
			results = append(results, FolderRunItemIO{
				Name:   name,
				Path:   path,
				Method: string(req.Method),
				Error:  execErr.Error(),
			})
			continue
		}
		results = append(results, FolderRunItemIO{
			Name:       name,
			Path:       path,
			Method:     string(req.Method),
			StatusCode: resp.StatusCode,
			DurationMs: resp.Duration.Milliseconds(),
			Passed:     resp.StatusCode >= 200 && resp.StatusCode < 300,
		})
	}
	return results
}

// RunScenario executes a scenario file and returns the full result.
// envPath overrides the workspace active environment; pass "" to use the active env.
// fromStep is the 0-based index of the first step to execute; pass 0 to run all steps.
func (s *ScenarioService) RunScenario(path, envPath string, fromStep int) ScenarioResultIO {
	data, err := os.ReadFile(path)
	if err != nil {
		return ScenarioResultIO{Error: fmt.Sprintf("read file: %v", err)}
	}
	sc, err := serialization.ParseScenarioFile(data, filepath.Base(path))
	if err != nil {
		return ScenarioResultIO{Error: fmt.Sprintf("parse: %v", err)}
	}

	// Resolve requestRef steps against the scenario's directory, then workspace requests/.
	scenDir := filepath.Dir(path)
	wsRequests := filepath.Join(s.workspace.workspacePath, "requests")
	for i := range sc.Steps {
		step := &sc.Steps[i]
		if step.Request != nil || step.RequestRef == "" {
			continue
		}
		candidates := []string{
			filepath.Join(scenDir, step.RequestRef),
			filepath.Join(wsRequests, step.RequestRef),
		}
		if filepath.IsAbs(step.RequestRef) {
			candidates = []string{step.RequestRef}
		}
		for _, refPath := range candidates {
			refData, err := os.ReadFile(refPath)
			if err != nil {
				continue
			}
			req, err := serialization.ParseRequestFile(refData, filepath.Base(refPath))
			if err == nil {
				step.Request = req
				break
			}
		}
	}

	exec := buildExecutorWithCookies(s.settings)

	// Load the OpenAPI spec now (before creating the runner) so that
	// type:schema assertions with an operationId value can resolve against it.
	var loadedSpec *openapi.Spec
	var specLoadErr string
	if sc.SpecFile != "" {
		specData, specExt, err := loadSpecBytes(sc.SpecFile, filepath.Dir(path))
		if err != nil {
			specLoadErr = fmt.Sprintf("specFile %q: %v", sc.SpecFile, err)
		} else {
			spec, err := openapi.ParseBytes(specData, specExt)
			if err != nil {
				specLoadErr = fmt.Sprintf("specFile %q: %v", sc.SpecFile, err)
			} else {
				loadedSpec = spec
			}
		}
	}

	var runner *scenario.Runner
	if loadedSpec != nil {
		runner = scenario.NewWithSpec(exec, loadedSpec)
	} else {
		runner = scenario.New(exec)
	}

	env := s.workspace.ActiveEnvironment()
	if envPath != "" {
		envData, err := os.ReadFile(envPath)
		if err != nil {
			return ScenarioResultIO{Error: fmt.Sprintf("read env file: %v", err)}
		}
		if loaded, err := serialization.ParseEnvironmentFile(envData, filepath.Base(envPath)); err == nil {
			env = loaded
		}
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.cancelRun = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.cancelRun = nil
		s.mu.Unlock()
	}()

	result := runner.RunFrom(ctx, sc, env, fromStep)
	s.rememberRun(path, result)

	out := ScenarioResultIO{
		Title:      result.Scenario.Title,
		Passed:     result.Passed,
		DurationMs: result.Duration.Milliseconds(),
		SpecError:  specLoadErr,
		Error:      redact.Credentials(result.SetupError),
	}
	if result.UsedRandomness {
		seed := result.Seed
		out.Seed = &seed
	}
	for _, v := range result.Variables {
		val := v.Value
		if v.Secret {
			val = "***"
		}
		val = redact.Credentials(val)
		out.Variables = append(out.Variables, VariableIO{Name: v.Name, Value: val, Secret: v.Secret})
	}
	for _, sr := range result.Steps {
		step := StepResultIO{
			Label:      sr.Label,
			Passed:     sr.Passed,
			Skipped:    sr.Skipped,
			Error:      redact.Credentials(sr.Error),
			Extracted:  sr.Extracted,
			Assertions: []AssertionResultIO{},
		}
		if sr.Attempts > 1 {
			step.Attempts = sr.Attempts
		}
		step.SpecState, step.SpecReason = string(sr.Spec.State), sr.Spec.Reason
		for _, v := range sr.Spec.Violations {
			step.SpecViolations = append(step.SpecViolations, v.Error())
		}
		step.SpecFindings = specFindings(sr, result.Secrets)
		if sr.Request != nil {
			step.RequestMethod, step.RequestURL, step.RequestHeaders, step.RequestBody = exchangeRequest(sr.Request, result.Secrets)
		}
		if sr.Response != nil {
			step.StatusCode = sr.Response.StatusCode
			step.DurationMs = sr.Response.Duration.Milliseconds()
			step.ResponseBody = sr.Response.BodyString()
			for _, h := range sr.Response.Headers {
				step.ResponseHeaders = append(step.ResponseHeaders, HeaderIO{Name: h.Name, Value: h.Value})
			}
			if step.ResponseHeaders == nil {
				step.ResponseHeaders = []HeaderIO{}
			}
		}
		for _, ar := range sr.Assertions {
			step.Assertions = append(step.Assertions, AssertionResultIO{
				Type:    string(ar.Assertion.Type),
				Passed:  ar.Passed,
				Message: ar.Message,
			})
		}
		if step.Extracted == nil {
			step.Extracted = make(map[string]string)
		}
		out.Steps = append(out.Steps, step)
	}

	// Compute spec coverage using the already-loaded spec (if any).
	if loadedSpec != nil {
		var executed []openapi.ExecutedRequest
		for _, sr := range result.Steps {
			if sr.Skipped || sr.Request == nil {
				continue
			}
			statusCode := 0
			var durationMs int64
			if sr.Response != nil {
				statusCode = sr.Response.StatusCode
				durationMs = sr.Response.Duration.Milliseconds()
			}
			executed = append(executed, openapi.ExecutedRequest{
				Method:     string(sr.Request.Method),
				URL:        sr.Request.URL,
				StatusCode: statusCode,
				HadAuth:    sr.Request.Auth != nil,
				Passed:     sr.Passed,
				DurationMs: durationMs,
			})
		}
		if cr := loadedSpec.ScenarioCoverage(executed); cr != nil {
			cov := &CoverageIO{
				Covered: cr.Covered,
				Total:   cr.Total,
				Percent: cr.Percent,
			}
			for _, op := range cr.Exercised {
				cov.Exercised = append(cov.Exercised, op.String())
			}
			for _, op := range cr.Missed {
				cov.Missed = append(cov.Missed, op.String())
			}
			out.Coverage = cov

			// Accumulate into the workspace coverage store.
			if s.workspace.workspacePath != "" {
				if acc, ok := accumulateCoverage(s.workspace.workspacePath, sc.SpecFile, loadedSpec, cr, result.Passed, filepath.Dir(path)); ok {
					out.AccumulatedCoverage = &acc
				}
			}
		}
	}

	// Record this run in the scenario run log (silently — a broken db never blocks results).
	s.recordScenarioRun(path, out)

	return out
}

// CancelFolderRun stops the RunFolder call in progress, if any.
func (s *ScenarioService) CancelFolderRun() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFolder != nil {
		s.cancelFolder()
	}
}

// CancelScenario aborts the currently in-flight RunScenario call, if any.
func (s *ScenarioService) CancelScenario() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelRun != nil {
		s.cancelRun()
		s.cancelRun = nil
	}
}

// recordScenarioRun opens the history database and appends a run record.
// Errors are silently discarded so a broken database never blocks scenario output.
func (s *ScenarioService) recordScenarioRun(scenarioPath string, out ScenarioResultIO) {
	dbPath, err := history.DefaultPath()
	if err != nil {
		return
	}
	store, err := history.Open(dbPath)
	if err != nil {
		return
	}
	defer store.Close()

	counts := result{passed: 0, failed: 0, skipped: 0}
	steps := make([]history.ScenarioStepSummary, len(out.Steps))
	for i, sr := range out.Steps {
		steps[i] = history.ScenarioStepSummary{
			Label:      sr.Label,
			Passed:     sr.Passed,
			Skipped:    sr.Skipped,
			StatusCode: sr.StatusCode,
			DurationMs: sr.DurationMs,
			Error:      sr.Error,
		}
		if sr.Skipped {
			counts.skipped++
		} else if sr.Passed {
			counts.passed++
		} else {
			counts.failed++
		}
	}

	_ = store.RecordScenarioRun(history.ScenarioRunRecord{
		WorkspacePath: s.workspace.workspacePath,
		ScenarioPath:  scenarioPath,
		Title:         out.Title,
		Passed:        out.Passed,
		DurationMs:    out.DurationMs,
		StepTotal:     len(out.Steps),
		StepPassed:    counts.passed,
		StepFailed:    counts.failed,
		StepSkipped:   counts.skipped,
		Steps:         steps,
	})
}

type result struct{ passed, failed, skipped int }

// GetScenarioRunHistory returns recent scenario runs for the current workspace.
func (s *ScenarioService) GetScenarioRunHistory(limit int) []ScenarioRunEntryIO {
	if limit <= 0 {
		limit = 100
	}
	dbPath, err := history.DefaultPath()
	if err != nil {
		return nil
	}
	store, err := history.Open(dbPath)
	if err != nil {
		return nil
	}
	defer store.Close()

	records, err := store.RecentScenarioRuns(s.workspace.workspacePath, limit)
	if err != nil {
		return nil
	}
	out := make([]ScenarioRunEntryIO, len(records))
	for i, r := range records {
		steps := make([]StepRunEntryIO, len(r.Steps))
		for j, sr := range r.Steps {
			steps[j] = StepRunEntryIO{
				Label:      sr.Label,
				Passed:     sr.Passed,
				Skipped:    sr.Skipped,
				StatusCode: sr.StatusCode,
				DurationMs: sr.DurationMs,
				Error:      sr.Error,
			}
		}
		out[i] = ScenarioRunEntryIO{
			ID:           r.ID,
			ExecutedAt:   r.ExecutedAt.Format(time.RFC3339),
			ScenarioPath: r.ScenarioPath,
			Title:        r.Title,
			Passed:       r.Passed,
			DurationMs:   r.DurationMs,
			StepTotal:    r.StepTotal,
			StepPassed:   r.StepPassed,
			StepFailed:   r.StepFailed,
			StepSkipped:  r.StepSkipped,
			Steps:        steps,
		}
	}
	return out
}

func (s *ScenarioService) rememberRun(path string, r *scenario.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs == nil {
		s.runs = map[string]*scenario.Result{}
	}
	s.runs[path] = r
}

// requestPath is the path part of a request URL, for pointing a finding at the operation.
func requestPath(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return u.Path
	}
	if i := strings.Index(raw, "?"); i >= 0 {
		return raw[:i]
	}
	return raw
}

// specFindings explains the spec violations of one step in plain language, as the Check Conformance
// page does for an operation. The operation is the one the step called, so its path is the request's.
func specFindings(sr scenario.StepResult, secrets []string) []ConformFindingIO {
	if len(sr.Spec.Violations) == 0 || sr.Request == nil {
		return nil
	}
	op := conformance.OperationResult{Method: string(sr.Request.Method), Path: requestPath(sr.Request.URL), Violations: sr.Spec.Violations}
	if sr.Response != nil {
		op.StatusCode = sr.Response.StatusCode
	}
	return findingsIO(reporting.Explain(op, secrets))
}
