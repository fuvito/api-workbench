package reporting

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/checks"
	"github.com/fuvito/api-workbench/packages/core/conformance"
	"github.com/fuvito/api-workbench/packages/core/openapi"
)

// The text and JSON reports of a conformance check and its negative probes.

// noProbesNote says why a run of negative probes tested nothing, so that it does not read as a pass.
func noProbesNote(skipped int) string {
	if skipped > 0 {
		return "no negative probe was sent: every probe of the selected operations was skipped"
	}
	return "no negative probe applies to the selected operations: nothing was tested (a probe needs a required body field or parameter, or an operation that asks for credentials)"
}

// ---- Conformance reporting ----

// severityMark is the sign in front of a violation line: ✗ for a failure, ! for a warning.
func severityMark(op conformance.OperationResult, v openapi.ValidationError, request bool) string {
	if conformance.ViolationSeverity(op, v, request) == checks.Warning {
		return "!"
	}
	return "✗"
}

// ConformText returns a human-readable conformance report suitable for terminal / CI output.
// Pass a non-nil coverage to append a coverage summary section.
func ConformText(result *conformance.ConformResult, coverage *conformance.CoverageReport) string {
	return ConformTextExplained(result, coverage, false)
}

// ConformTextExplained is ConformText that can also explain each failure in plain language (what
// the spec expected, what the server sent, why that fails and what to change). The explanation is
// opt-in so the default output, which scripts and CI logs rely on, stays as it was.
func ConformTextExplained(result *conformance.ConformResult, coverage *conformance.CoverageReport, explainFailures bool) string {
	var b strings.Builder
	x := newRedactor(result.Secrets, false)

	fmt.Fprintf(&b, "%s  %s\n", result.SpecTitle, result.SpecVersion)
	fmt.Fprintf(&b, "Base URL: %s\n", x.text(result.BaseURL))
	if line := result.Scope.Line(); line != "" {
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintln(&b)

	sent, unanswered, firstErr := 0, 0, ""
	kinds := map[string]bool{}
	for _, op := range result.Operations {
		if op.Skipped || op.Blocked != "" {
			continue
		}
		sent++
		if op.Error != "" {
			unanswered++
			kinds[conformance.NetworkErrorKind(op.Error)] = true
			if firstErr == "" {
				firstErr = op.Error
			}
		}
	}
	nobodyAnswered := sent > 0 && unanswered == sent
	// One line for a dead server is enough; when the operations fail for different reasons (a
	// timeout here, a dropped body there) each keeps its own error.
	serverDown := nobodyAnswered && len(kinds) == 1 && kinds[conformance.NetUnreachable]

	for _, op := range result.Operations {
		path := op.ReportPath()
		if op.Skipped {
			fmt.Fprintf(&b, "  SKIP  %-7s %s\n", op.Method, path)
			continue
		}
		status := "PASS"
		if op.Error != "" || !result.OpPassed(op) {
			status = "FAIL"
		} else if len(op.Warnings()) > 0 {
			status = "WARN"
		}
		suffix := ""
		if op.StatusCode > 0 {
			suffix = fmt.Sprintf("  %d  %dms", op.StatusCode, op.DurationMs)
		}
		if len(op.RequestViolations) > 0 && len(op.Violations) == 0 && op.Error == "" {
			suffix += "  (the request does not fit the spec; the answer was not judged)"
		}
		fmt.Fprintf(&b, "  %s  %-7s %s%s\n", status, op.Method, path, suffix)
		if op.Error != "" && !serverDown {
			fmt.Fprintf(&b, "         error: %s\n", x.text(op.Error))
		}
		if op.Blocked != "" {
			fmt.Fprintf(&b, "         ✗ %s\n", x.text(op.Blocked))
		}
		for _, v := range op.RequestViolations {
			fmt.Fprintf(&b, "         %s request: %s\n", severityMark(op, v, true), x.text(v.Error()))
		}
		for _, v := range op.Violations {
			fmt.Fprintf(&b, "         %s %s\n", severityMark(op, v, false), x.text(v.Error()))
		}
		if explainFailures {
			for _, f := range explain(op, x) {
				fmt.Fprintf(&b, "         → %s\n", explanationLine(f))
			}
		}
	}

	fmt.Fprintln(&b)
	switch {
	case serverDown:
		fmt.Fprintf(&b, "No operation got an answer from %s: is the server running?\n  First error: %s\n\n", x.text(result.BaseURL), x.text(firstErr))
	case nobodyAnswered:
		fmt.Fprintf(&b, "No operation got an answer from %s (the error under each operation says why).\n\n", x.text(result.BaseURL))
	}
	passed, failed, skipped := result.Counts()
	total := passed + failed + skipped

	verdict := "PASSED"
	switch {
	case passed+failed == 0:
		verdict = "NOTHING CHECKED"
	case failed > 0 && result.BaselineApplied && result.BaselineClean:
		verdict = fmt.Sprintf("PASSED with %d known violation%s", result.KnownViolations, plural(result.KnownViolations))
	case failed > 0:
		verdict = "FAILED"
	case result.WarningCount() > 0:
		verdict = fmt.Sprintf("PASSED with %d warning%s", result.WarningCount(), plural(result.WarningCount()))
	}
	fmt.Fprintf(&b, "%s  %s  %dms\n", verdict, conformTally(result), result.Duration.Milliseconds())
	if passed+failed == 0 {
		if total == 0 {
			fmt.Fprintf(&b, "       The spec declares no operations, so there was nothing to check.\n")
		} else if result.Scope != nil {
			fmt.Fprintf(&b, "       Nothing was sent: no operation changed against %s.\n", result.Scope.ChangedAgainst)
		} else {
			fmt.Fprintf(&b, "       Every operation was skipped (by --tags, --paths or --read-only), so nothing was sent.\n")
		}
	}
	if n, missing := result.BlockedSummary(); n > 0 {
		fmt.Fprintf(&b, "%s\n", conformance.MissingValuesAdvice(n, missing))
	}

	if coverage != nil {
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "Coverage: %d / %d operations (%.0f%%)\n", coverage.Covered, coverage.Total, coverage.Percent)
		if len(coverage.CoveredOps) > 0 {
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "Covered:")
			for _, op := range coverage.CoveredOps {
				fmt.Fprintf(&b, "  %-7s %s\n", op.Method, op.ReportPath())
			}
		}
		if len(coverage.NotCoveredOps) > 0 {
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "Not covered:")
			for _, op := range coverage.NotCoveredOps {
				fmt.Fprintf(&b, "  %-7s %s\n", op.Method, op.ReportPath())
			}
		}
	}

	return b.String()
}

// JSONConformReport is the machine-readable structure for a conformance run.
type JSONConformReport struct {
	SpecTitle   string `json:"specTitle"`
	SpecVersion string `json:"specVersion"`
	BaseURL     string `json:"baseURL"`
	Passed      bool   `json:"passed"`
	// Warnings is how many operations passed with warnings only.
	Warnings   int   `json:"warnings,omitempty"`
	DurationMs int64 `json:"durationMs"`
	// Scope is set when the run checked only the operations a change touched.
	Scope *conformance.Scope `json:"scope,omitempty"`
	// MissingValues lists the placeholders that kept operations from being sent (each once).
	MissingValues []string        `json:"missingValues,omitempty"`
	Operations    []JSONConformOp `json:"operations"`
	Coverage      *JSONCoverage   `json:"coverage,omitempty"`
	// Probes lists the negative probes (only with --negative): each one is a request that breaks
	// the spec on purpose and should be answered with a 4xx.
	Probes []JSONConformProbe `json:"probes,omitempty"`
}

// JSONConformProbe is one negative probe in a conformance JSON report.
type JSONConformProbe struct {
	OperationID string `json:"operationId,omitempty"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	StatusCode  int    `json:"statusCode,omitempty"`
	DurationMs  int64  `json:"durationMs,omitempty"`
	Passed      bool   `json:"passed"`
	Skipped     bool   `json:"skipped,omitempty"`
	Error       string `json:"error,omitempty"`
	// NotChecked: a rate limit left the probe unjudged. CheckID and Warning say what is doubtful
	// about an answer; both are absent for a plain pass or failure.
	NotChecked bool   `json:"notChecked,omitempty"`
	CheckID    string `json:"checkId,omitempty"`
	Warning    string `json:"warning,omitempty"`
}

type JSONConformOp struct {
	OperationID string `json:"operationId,omitempty"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	// SpecPath is the path as the spec writes it (/users/{id}); Path is the path of the generated
	// request (/users/{{id}}) and is unchanged.
	SpecPath   string `json:"specPath"`
	StatusCode int    `json:"statusCode,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	Passed     bool   `json:"passed"`
	// Warnings counts the findings of this operation that are only warnings.
	Warnings int    `json:"warnings,omitempty"`
	Skipped  bool   `json:"skipped,omitempty"`
	Error    string `json:"error,omitempty"`
	// Blocked says why the request was not sent (a missing value); Missing names the placeholders.
	Blocked           string   `json:"blocked,omitempty"`
	Missing           []string `json:"missing,omitempty"`
	RequestViolations []string `json:"requestViolations,omitempty"`
	Violations        []string `json:"violations,omitempty"`
	// Findings explain each failure in plain language (added in addition to the fields above,
	// which are unchanged).
	Findings []JSONConformFinding `json:"findings,omitempty"`
}

// JSONConformFinding is the machine-readable form of conformance.Finding.
type JSONConformFinding struct {
	Kind string `json:"kind"`
	// CheckID is the check's ID in the catalog (for example CS-RESP-005); absent when the violation
	// is not recognised.
	CheckID string `json:"checkId,omitempty"`
	// Severity is failure, warning or note.
	Severity string `json:"severity,omitempty"`
	// Part and Where say where in the response the problem is (a header name, the status, a body
	// field); Spec says where the expectation comes from in the spec.
	Part     string `json:"part,omitempty"`
	Where    string `json:"where,omitempty"`
	Spec     string `json:"spec,omitempty"`
	Summary  string `json:"summary"`
	Expected string `json:"expected,omitempty"`
	Got      string `json:"got,omitempty"`
	Why      string `json:"why"`
	Fix      string `json:"fix"`
}

// JSONCoverage is the machine-readable coverage summary.
type JSONCoverage struct {
	Total         int              `json:"total"`
	Covered       int              `json:"covered"`
	NotCovered    int              `json:"notCovered"`
	Percent       float64          `json:"percent"`
	CoveredOps    []JSONCoverageOp `json:"coveredOps"`
	NotCoveredOps []JSONCoverageOp `json:"notCoveredOps"`
}

// JSONCoverageOp identifies a single operation in a coverage list.
type JSONCoverageOp struct {
	OperationID string `json:"operationId,omitempty"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	// SpecPath is the path as the spec writes it; set in conformance reports.
	SpecPath string `json:"specPath,omitempty"`
}

// ConformJSON returns a machine-readable JSON conformance report.
// Pass a non-nil coverage to include a coverage summary in the output and a non-nil neg to include
// the negative probes (a failed probe makes the report not passed, as it does the exit code).
func ConformJSON(result *conformance.ConformResult, coverage *conformance.CoverageReport, neg *conformance.NegativeResult) ([]byte, error) {
	_, failed, _ := result.Counts()
	if neg != nil {
		_, _, _, negErrored := neg.Counts()
		failed += neg.FailingProbes() + negErrored
	}
	x := newRedactor(result.Secrets, false)
	report := JSONConformReport{
		SpecTitle:   result.SpecTitle,
		SpecVersion: result.SpecVersion,
		BaseURL:     x.text(result.BaseURL),
		Scope:       result.Scope,
		Passed:      failed == 0,
		DurationMs:  result.Duration.Milliseconds(),
	}
	_, report.MissingValues = result.BlockedSummary()
	report.Warnings = result.WarningCount()
	for _, op := range result.Operations {
		jo := JSONConformOp{
			OperationID: op.OperationID,
			Method:      op.Method,
			Path:        op.Path,
			SpecPath:    op.ReportPath(),
			StatusCode:  op.StatusCode,
			DurationMs:  op.DurationMs,
			Passed:      result.OpPassed(op),
			Warnings:    len(op.Warnings()),
			Skipped:     op.Skipped,
			Error:       x.text(op.Error),
			Blocked:     x.text(op.Blocked),
			Missing:     op.Missing,
		}
		for _, v := range op.RequestViolations {
			jo.RequestViolations = append(jo.RequestViolations, x.text(v.Error()))
		}
		for _, v := range op.Violations {
			jo.Violations = append(jo.Violations, x.text(v.Error()))
		}
		for _, f := range explain(op, x) {
			jo.Findings = append(jo.Findings, JSONConformFinding{
				Kind: f.Kind, CheckID: f.CheckID, Severity: string(f.Severity), Part: f.Part, Where: f.Where, Spec: f.Spec, Summary: f.Summary, Expected: f.Expected, Got: f.Got, Why: f.Why, Fix: f.Fix,
			})
		}
		report.Operations = append(report.Operations, jo)
	}
	if neg != nil {
		for _, p := range neg.Probes {
			report.Probes = append(report.Probes, JSONConformProbe{
				OperationID: p.OperationID, Method: p.Method, Path: p.Path, Kind: string(p.Kind), Description: x.text(p.Description),
				StatusCode: p.StatusCode, DurationMs: p.DurationMs, Passed: !p.Skipped && p.Error == "" && p.Passed(),
				Skipped: p.Skipped, Error: x.text(p.Error),
				NotChecked: p.RateLimited(), CheckID: p.WarningCheck(), Warning: p.Warning(),
			})
		}
	}
	if coverage != nil {
		jc := &JSONCoverage{
			Total:         coverage.Total,
			Covered:       coverage.Covered,
			NotCovered:    coverage.NotCovered,
			Percent:       coverage.Percent,
			CoveredOps:    make([]JSONCoverageOp, 0, len(coverage.CoveredOps)),
			NotCoveredOps: make([]JSONCoverageOp, 0, len(coverage.NotCoveredOps)),
		}
		for _, op := range coverage.CoveredOps {
			jc.CoveredOps = append(jc.CoveredOps, JSONCoverageOp{OperationID: op.OperationID, Method: op.Method, Path: op.Path, SpecPath: op.ReportPath()})
		}
		for _, op := range coverage.NotCoveredOps {
			jc.NotCoveredOps = append(jc.NotCoveredOps, JSONCoverageOp{OperationID: op.OperationID, Method: op.Method, Path: op.Path, SpecPath: op.ReportPath()})
		}
		report.Coverage = jc
	}
	return json.MarshalIndent(report, "", "  ")
}

// conformCounts splits what Counts calls failed into operations that were sent and failed and
// operations that were never sent for lack of a value. Both still fail the run.
func conformCounts(result *conformance.ConformResult) (passed, failed, notSent, skipped int) {
	passed, failed, skipped = result.Counts()
	notSent, _ = result.BlockedSummary()
	return passed, failed - notSent, notSent, skipped
}

// conformTally is the counts line of the text, HTML and Markdown reports. "not sent" appears only
// when an operation was not sent, so the line of any other run reads as it always has.
func conformTally(result *conformance.ConformResult) string {
	passed, failed, notSent, skipped := conformCounts(result)
	parts := []string{
		fmt.Sprintf("%d operations", passed+failed+notSent+skipped),
		fmt.Sprintf("%d passed", passed),
		fmt.Sprintf("%d failed", failed),
	}
	if notSent > 0 {
		parts = append(parts, fmt.Sprintf("%d not sent", notSent))
	}
	return strings.Join(append(parts, fmt.Sprintf("%d skipped", skipped)), "  ·  ")
}

// NegativeText returns a human-readable negative conformance report.
func NegativeText(result *conformance.NegativeResult) string {
	var b strings.Builder
	x := newRedactor(result.Secrets, false)

	fmt.Fprintf(&b, "\nNegative conformance probes — %s %s\n", result.SpecTitle, result.SpecVersion)

	for _, p := range result.Probes {
		switch {
		case p.Skipped:
			fmt.Fprintf(&b, "  SKIP  %s\n", p.Description)
		case p.Error != "":
			fmt.Fprintf(&b, "  ERR   %s\n         error: %s\n", p.Description, x.text(p.Error))
		case p.RateLimited():
			fmt.Fprintf(&b, "  ----  %s\n         %s %s\n", p.Description, p.Warning(), p.WarningCheck())
		case p.Passed() && p.WarningCheck() != "":
			fmt.Fprintf(&b, "  WARN  %s  %d  %dms\n         %s %s\n", p.Description, p.StatusCode, p.DurationMs, p.Warning(), p.WarningCheck())
		case p.Passed():
			fmt.Fprintf(&b, "  PASS  %s  %d  %dms\n", p.Description, p.StatusCode, p.DurationMs)
		default:
			fmt.Fprintf(&b, "  FAIL  %s\n         server accepted invalid request with %d\n", p.Description, p.StatusCode)
		}
	}

	fmt.Fprintln(&b)
	passed, failed, skipped, errored := result.Counts()
	total := passed + failed + errored
	otherReason, rateLimited := result.Tally()

	verdict := "PASSED"
	switch {
	case result.FailingProbes() > 0 || errored > 0:
		verdict = "FAILED"
	case total == 0:
		verdict = "NOTHING CHECKED"
	case otherReason+rateLimited > 0:
		verdict = fmt.Sprintf("PASSED with %d warning%s", otherReason+rateLimited, plural(otherReason+rateLimited))
	}
	unchecked := ""
	if rateLimited > 0 {
		unchecked = fmt.Sprintf("  ·  %d not checked", rateLimited)
	}
	fmt.Fprintf(&b, "%s  %d probe%s  ·  %d passed  ·  %d failed%s  ·  %d skipped  ·  %d error%s  %dms\n",
		verdict, total, plural(total), passed, failed, unchecked, skipped, errored, plural(errored), result.Duration.Milliseconds())
	if otherReason > 0 {
		fmt.Fprintf(&b, "       %d of the passed probe%s: refused with a status that does not show why — see %s\n", otherReason, plural(otherReason), conformance.CheckProbeOtherReason)
	}
	if rateLimited > 0 {
		fmt.Fprintf(&b, "       %d probe%s not checked: the server limited the rate (429) — run again more slowly or raise the limit for this client\n", rateLimited, plural(rateLimited))
	}
	if failed > 0 {
		authFailed, otherFailed := 0, 0
		for _, p := range result.Probes {
			if !p.Failed() {
				continue
			}
			if p.Kind == openapi.ProbeMissingAuth {
				authFailed++
			} else {
				otherFailed++
			}
		}
		if otherFailed > 0 {
			fmt.Fprintf(&b, "       %d probe%s: the server accepted a request it should have rejected — fix server validation\n", otherFailed, plural(otherFailed))
		}
		if authFailed > 0 {
			fmt.Fprintf(&b, "       %d probe%s sent no credentials and the server still accepted the request — fix the authentication rule for those operations\n", authFailed, plural(authFailed))
		}
	}
	if total == 0 && rateLimited == 0 {
		fmt.Fprintf(&b, "       %s\n", noProbesNote(skipped))
	}
	if errored > 0 {
		fmt.Fprintf(&b, "       %d probe(s) got no answer, so they could not be judged — check that the server is running and the base URL is right\n", errored)
	}

	return b.String()
}
