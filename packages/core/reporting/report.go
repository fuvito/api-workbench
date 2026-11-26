package reporting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/redact"
	"github.com/fuvito/api-workbench/packages/core/scenario"
)

// Text returns a human-readable scenario result suitable for terminal output.
// Uses plain ASCII (no ANSI codes) so it is safe in CI logs.
func Text(result *scenario.Result) string { return textScenario(result, true) }

// textScenario is Text; showSeed is false when several scenarios share one run seed that the
// combined report prints once.
func textScenario(result *scenario.Result, showSeed bool) string {
	var b strings.Builder
	x := redactorFor(result)

	fmt.Fprintf(&b, "Scenario: %s\n", result.Scenario.Title)
	if result.Scenario.Description != "" {
		fmt.Fprintf(&b, "          %s\n", result.Scenario.Description)
	}
	if showSeed && result.UsedRandomness {
		fmt.Fprintf(&b, "          seed: %d\n", result.Seed)
	}
	for _, v := range result.Variables {
		fmt.Fprintf(&b, "          var %s = %s\n", v.Name, x.variable(v))
	}
	if result.SetupError != "" {
		fmt.Fprintf(&b, "          error: %s\n", x.text(result.SetupError))
	}
	fmt.Fprintln(&b)

	for _, sr := range result.Steps {
		if sr.Skipped {
			fmt.Fprintf(&b, "  SKIP  %s\n", sr.Label)
			if sr.SkipReason != "" {
				fmt.Fprintf(&b, "         skipped: %s\n", sr.SkipReason)
			}
			continue
		}

		status := "PASS"
		if sr.Error != "" || !sr.Passed {
			status = "FAIL"
		}

		suffix := ""
		if sr.Response != nil {
			suffix = fmt.Sprintf("  %d  %dms", sr.Response.StatusCode, sr.Response.Duration.Milliseconds())
		}

		if sr.Attempts > 1 {
			suffix += fmt.Sprintf("  (%d attempts)", sr.Attempts)
		}
		fmt.Fprintf(&b, "  %s  %s%s\n", status, sr.Label, suffix)

		if sr.Error != "" {
			fmt.Fprintf(&b, "         error: %s\n", x.text(sr.Error))
		}

		for _, ar := range sr.Assertions {
			mark := "✓"
			if !ar.Passed {
				mark = "✗"
			}
			label := assertionLabel(ar.Assertion)
			if ar.Passed {
				fmt.Fprintf(&b, "         %s %s\n", mark, label)
			} else {
				fmt.Fprintf(&b, "         %s %s: %s\n", mark, label, x.text(ar.Message))
				if f, ok := schemaFinding(ar); ok {
					fmt.Fprintf(&b, "         → %s\n", x.text(explanationLine(f)))
				}
			}
		}

		shown := x.extracted(sr.Extracted)
		for _, name := range sortedKeys(shown) {
			fmt.Fprintf(&b, "         → %s = %q\n", name, shown[name])
		}
	}

	fmt.Fprintln(&b)
	if line := notComparedLine(result); line != "" {
		fmt.Fprintln(&b, line)
	}
	passed, failed, skipped := result.Counts()
	apassed, afailed := result.AssertionCounts()

	verdict := "PASSED"
	if !result.Passed {
		verdict = "FAILED"
	}
	fmt.Fprintf(&b, "%s  %d passed, %d failed, %d skipped  (%d/%d assertions passed)  %dms\n",
		verdict, passed, failed, skipped, apassed, apassed+afailed, result.Duration.Milliseconds())

	return b.String()
}

// notComparedLine names the steps whose response was not compared with the scenario's spec and
// why, so a run that checked nothing cannot be read as a run that found nothing. It is empty when
// every step was compared (or the scenario has no spec).
func notComparedLine(result *scenario.Result) string {
	reasons := map[string]int{}
	total := 0
	for _, sr := range result.Steps {
		if sr.Spec.State == "" {
			continue
		}
		total++
		if sr.Spec.State == scenario.SpecNotChecked {
			reasons[sr.Spec.Reason]++
		}
	}
	if len(reasons) == 0 {
		return ""
	}
	names := make([]string, 0, len(reasons))
	n := 0
	for r, c := range reasons {
		names = append(names, fmt.Sprintf("%s: %d", r, c))
		n += c
	}
	sort.Strings(names)
	return fmt.Sprintf("Spec: %d of %d responses were not compared with the spec (%s)", n, total, strings.Join(names, "; "))
}

// expectedText is the expected value of an assertion as text; empty when it has none, such as a
// schema assertion.
func expectedText(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// assertionLabel names an assertion in a report. The expected value it shows may hold a password.
func assertionLabel(a domain.Assertion) string { return redact.Credentials(rawAssertionLabel(a)) }

func rawAssertionLabel(a domain.Assertion) string {
	op := string(a.Operator)
	if op == "" {
		op = "eq"
	}
	switch a.Type {
	case domain.AssertionStatus:
		return fmt.Sprintf("status %s %v", op, a.Expected)
	case domain.AssertionHeader:
		return fmt.Sprintf("header %s %s %v", a.Path, op, a.Expected)
	case domain.AssertionJSONPath:
		return fmt.Sprintf("jsonpath %s %s %v", a.Path, op, a.Expected)
	case domain.AssertionSchema:
		return "schema"
	case domain.AssertionDuration:
		return fmt.Sprintf("duration %s %v ms", op, a.Expected)
	default:
		if a.Name != "" {
			return a.Name
		}
		return string(a.Type)
	}
}

// TextWithCoverage is like Text but appends a spec coverage section.
func TextWithCoverage(result *scenario.Result, cov *openapi.ScenarioCoverageResult) string {
	base := Text(result)
	if cov == nil {
		return base
	}
	return base + "\n" + CoverageText(cov, "")
}

func sortedCovOps(ops []openapi.CoveredOperation) []openapi.CoveredOperation {
	out := make([]openapi.CoveredOperation, len(ops))
	copy(out, ops)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}
