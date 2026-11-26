package reporting

import (
	"encoding/json"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/conformance"
	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/scenario"
)

// The JSON report of a scenario run.

// JSONReport is the machine-readable output structure for CI/CD pipelines.
type JSONReport struct {
	Scenario   string                `json:"scenario"`
	Passed     bool                  `json:"passed"`
	DurationMs int64                 `json:"durationMs"`
	Seed       *int64                `json:"seed,omitempty"`
	Error      string                `json:"error,omitempty"`
	Variables  []JSONVariable        `json:"variables,omitempty"`
	Steps      []JSONStep            `json:"steps"`
	Coverage   *JSONScenarioCoverage `json:"coverage,omitempty"`
}

// JSONScenarioCoverage is the coverage section in a scenario JSON report.
type JSONScenarioCoverage struct {
	Total     int              `json:"total"`
	Covered   int              `json:"covered"`
	Percent   float64          `json:"percent"`
	Exercised []JSONCoverageOp `json:"exercised"`
	Missed    []JSONCoverageOp `json:"missed"`
}

// JSONVariable is a scenario variable and the value it was given for the run.
type JSONVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type JSONStep struct {
	Name       string            `json:"name"`
	Passed     bool              `json:"passed"`
	Skipped    bool              `json:"skipped,omitempty"`
	Error      string            `json:"error,omitempty"`
	StatusCode int               `json:"statusCode,omitempty"`
	DurationMs int64             `json:"durationMs,omitempty"`
	Attempts   int               `json:"attempts,omitempty"` // only when the step was re-sent
	Extracted  map[string]string `json:"extracted,omitempty"`
	Assertions []JSONAssertion   `json:"assertions,omitempty"`
}

type JSONAssertion struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Path     string `json:"path,omitempty"`
	Operator string `json:"operator,omitempty"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message,omitempty"`
	// CheckID names the catalog check a failed schema assertion maps to (for example CS-RESP-005).
	CheckID string `json:"checkId,omitempty"`
}

// schemaFinding explains a failed `schema` assertion with the same check IDs, why and fix text as
// conform, so a scenario result can be found in the catalog. It reports false for any other
// assertion, for a pass, and when the message is not a recognised schema violation.
func schemaFinding(ar domain.AssertionResult) (conformance.Finding, bool) {
	if ar.Passed || ar.Assertion.Type != domain.AssertionSchema || ar.Message == "" {
		return conformance.Finding{}, false
	}
	msg := ar.Message
	if i := strings.Index(msg, "; Error at"); i > 0 {
		msg = msg[:i]
	}
	fs := conformance.Diagnose(conformance.OperationResult{Violations: []openapi.ValidationError{{Message: msg}}})
	if len(fs) == 0 || fs[0].CheckID == "" {
		return conformance.Finding{}, false
	}
	f := fs[0]
	f.Spec = ""
	return f, true
}

// JSONWithCoverage is like JSON but includes spec coverage data.
func JSONWithCoverage(result *scenario.Result, cov *openapi.ScenarioCoverageResult) ([]byte, error) {
	b, err := JSON(result)
	if err != nil || cov == nil {
		return b, err
	}
	// Unmarshal → attach coverage → re-marshal.
	var report JSONReport
	if err := json.Unmarshal(b, &report); err != nil {
		return b, nil // shouldn't happen; return plain report
	}
	report.Coverage = jsonCoverage(cov)
	return json.MarshalIndent(report, "", "  ")
}

// JSON returns a machine-readable JSON report.
func JSON(result *scenario.Result) ([]byte, error) {
	x := redactorFor(result)
	report := JSONReport{
		Scenario:   result.Scenario.Title,
		Passed:     result.Passed,
		DurationMs: result.Duration.Milliseconds(),
	}

	if result.UsedRandomness {
		seed := result.Seed
		report.Seed = &seed
	}
	report.Error = x.text(result.SetupError)
	for _, v := range result.Variables {
		report.Variables = append(report.Variables, JSONVariable{Name: v.Name, Value: x.variable(v)})
	}
	for _, sr := range result.Steps {
		step := JSONStep{
			Name:      sr.Label,
			Passed:    sr.Passed,
			Skipped:   sr.Skipped,
			Error:     x.text(sr.Error),
			Extracted: x.extracted(sr.Extracted),
		}
		if sr.Response != nil {
			step.StatusCode = sr.Response.StatusCode
			step.DurationMs = sr.Response.Duration.Milliseconds()
		}
		if sr.Attempts > 1 {
			step.Attempts = sr.Attempts
		}
		for _, ar := range sr.Assertions {
			step.Assertions = append(step.Assertions, JSONAssertion{
				Type:     string(ar.Assertion.Type),
				Name:     ar.Assertion.Name,
				Path:     ar.Assertion.Path,
				Operator: string(ar.Assertion.Operator),
				Expected: x.text(expectedText(ar.Assertion.Expected)),
				Actual:   x.text(ar.Actual),
				Passed:   ar.Passed,
				Message:  x.text(ar.Message),
				CheckID:  schemaCheckID(ar),
			})
		}
		report.Steps = append(report.Steps, step)
	}

	return json.MarshalIndent(report, "", "  ")
}

func schemaCheckID(ar domain.AssertionResult) string {
	if f, ok := schemaFinding(ar); ok {
		return f.CheckID
	}
	return ""
}
