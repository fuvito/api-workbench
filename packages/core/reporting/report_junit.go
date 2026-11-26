package reporting

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/checks"
	"github.com/fuvito/api-workbench/packages/core/conformance"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/scenario"
)

// JUnit XML for scenario runs, conformance checks and spec diffs.

// ---- JUnit XML ----

type jUnitTestSuites struct {
	XMLName    xml.Name     `xml:"testsuites"`
	Name       string       `xml:"name,attr"`
	Tests      int          `xml:"tests,attr"`
	Failures   int          `xml:"failures,attr"`
	Errors     int          `xml:"errors,attr"`
	Time       string       `xml:"time,attr"`
	TestSuites []jUnitSuite `xml:"testsuite"`
}

type jUnitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type jUnitProperties struct {
	Property []jUnitProperty `xml:"property"`
}

type jUnitSuite struct {
	Name       string           `xml:"name,attr"`
	Tests      int              `xml:"tests,attr"`
	Failures   int              `xml:"failures,attr"`
	Errors     int              `xml:"errors,attr"`
	Time       string           `xml:"time,attr"`
	Properties *jUnitProperties `xml:"properties,omitempty"`
	Cases      []jUnitCase      `xml:"testcase"`
}

type jUnitCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Skipped   *struct{}     `xml:"skipped"`
	Failure   *jUnitFailure `xml:"failure"`
	Error     *jUnitError   `xml:"error"`
	// SystemOut carries the warnings of a passing test case (JUnit has no warning status).
	SystemOut string `xml:"system-out,omitempty"`
}

type jUnitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type jUnitError struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

func secStr(ms int64) string { return fmt.Sprintf("%.3f", float64(ms)/1000.0) }

// JUnit returns a JUnit-compatible XML report.
// Supported by Jenkins (Publish JUnit Results), GitHub Actions (dorny/test-reporter),
// GitLab CI (junit artifact), and CircleCI (store_test_results).
func JUnit(result *scenario.Result) ([]byte, error) {
	suite := junitSuiteFor(result)
	return marshalJUnit(jUnitTestSuites{
		Name:       result.Scenario.Title,
		Tests:      suite.Tests,
		Failures:   suite.Failures,
		Errors:     suite.Errors,
		Time:       suite.Time,
		TestSuites: []jUnitSuite{suite},
	})
}

// JUnitMulti returns one JUnit document with a test suite per scenario, for runs that
// execute several scenarios (for example a bundle). name labels the whole document.
func JUnitMulti(name string, results []*scenario.Result) ([]byte, error) {
	report := jUnitTestSuites{Name: name}
	var totalMs int64
	for _, r := range results {
		suite := junitSuiteFor(r)
		report.Tests += suite.Tests
		report.Failures += suite.Failures
		report.Errors += suite.Errors
		totalMs += r.Duration.Milliseconds()
		report.TestSuites = append(report.TestSuites, suite)
	}
	report.Time = secStr(totalMs)
	return marshalJUnit(report)
}

func marshalJUnit(report jUnitTestSuites) ([]byte, error) {
	out, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func junitSuiteFor(result *scenario.Result) jUnitSuite {
	_, failed, _ := result.Counts()
	x := redactorFor(result)

	suite := jUnitSuite{
		Name:     result.Scenario.Title,
		Tests:    len(result.Steps),
		Failures: failed,
		Time:     secStr(result.Duration.Milliseconds()),
	}

	if result.UsedRandomness {
		suite.Properties = &jUnitProperties{Property: []jUnitProperty{{Name: "seed", Value: strconv.FormatInt(result.Seed, 10)}}}
	}
	if result.SetupError != "" {
		suite.Tests++
		suite.Errors++
		suite.Cases = append(suite.Cases, jUnitCase{
			Name: "scenario variables", ClassName: result.Scenario.Title, Time: "0.000",
			Error: &jUnitError{Message: x.text(result.SetupError), Text: x.text(result.SetupError)},
		})
	}
	for _, sr := range result.Steps {
		stepMs := int64(0)
		if sr.Response != nil {
			stepMs = sr.Response.Duration.Milliseconds()
		}
		tc := jUnitCase{
			Name:      sr.Label,
			ClassName: result.Scenario.Title,
			Time:      secStr(stepMs),
		}
		switch {
		case sr.Skipped:
			skipped := struct{}{}
			tc.Skipped = &skipped
		case sr.Error != "":
			suite.Errors++
			tc.Error = &jUnitError{Message: x.text(sr.Error), Text: x.text(sr.Error)}
		case !sr.Passed:
			var lines []string
			for _, ar := range sr.Assertions {
				if !ar.Passed {
					lines = append(lines, fmt.Sprintf("  ✗ %s: %s", assertionLabel(ar.Assertion), x.text(ar.Message)))
				}
			}
			tc.Failure = &jUnitFailure{
				Message: fmt.Sprintf("%d assertion(s) failed", len(lines)),
				Text:    strings.Join(lines, "\n"),
			}
		}
		suite.Cases = append(suite.Cases, tc)
	}
	return suite
}

// ConformJUnit returns a JUnit XML conformance report. A non-nil neg adds one test case per
// negative probe.
func ConformJUnit(result *conformance.ConformResult, neg *conformance.NegativeResult) ([]byte, error) {
	_, failed, _ := result.Counts()

	suite := jUnitSuite{
		Name:     result.SpecTitle,
		Tests:    len(result.Operations),
		Failures: failed,
		Time:     secStr(result.Duration.Milliseconds()),
	}

	x := newRedactor(result.Secrets, false)
	for _, op := range result.Operations {
		tc := jUnitCase{
			Name:      op.Method + " " + op.ReportPath(),
			ClassName: result.SpecTitle,
			Time:      secStr(op.DurationMs),
		}
		switch {
		case op.Skipped:
			skipped := struct{}{}
			tc.Skipped = &skipped
		case op.Error != "":
			suite.Errors++
			tc.Error = &jUnitError{Message: x.text(op.Error), Text: x.text(op.Error)}
		case !result.OpPassed(op):
			var msgs []string
			if op.Blocked != "" {
				msgs = append(msgs, "  ✗ "+x.text(op.Blocked))
			}
			for _, v := range op.RequestViolations {
				msgs = append(msgs, "  ✗ request: "+x.text(v.Error()))
			}
			for _, v := range op.Violations {
				msgs = append(msgs, "  ✗ "+x.text(v.Error()))
			}
			violations := len(msgs)
			for _, f := range explain(op, x) {
				msgs = append(msgs, "  → "+explanationLine(f))
			}
			tc.Failure = &jUnitFailure{
				Message: fmt.Sprintf("%d violation(s)", violations),
				Text:    strings.Join(msgs, "\n"),
			}
		default:
			var lines []string
			for _, f := range explain(op, x) {
				if f.Severity == checks.Warning {
					lines = append(lines, "warning: "+explanationLine(f))
				}
			}
			tc.SystemOut = strings.Join(lines, "\n")
		}
		suite.Cases = append(suite.Cases, tc)
	}

	if neg != nil {
		for _, p := range neg.Probes {
			tc := jUnitCase{Name: "probe: " + x.text(p.Description), ClassName: result.SpecTitle, Time: secStr(p.DurationMs)}
			suite.Tests++
			switch {
			case p.Skipped:
				skipped := struct{}{}
				tc.Skipped = &skipped
			case p.Error != "":
				suite.Errors++
				tc.Error = &jUnitError{Message: x.text(p.Error), Text: x.text(p.Error)}
			case p.Failed():
				suite.Failures++
				msg := fmt.Sprintf("the server accepted an invalid request with %d", p.StatusCode)
				tc.Failure = &jUnitFailure{Message: msg, Text: msg}
			case p.FailsRun(neg.FailOn):
				suite.Failures++
				msg := p.WarningCheck() + " " + p.Warning()
				tc.Failure = &jUnitFailure{Message: msg, Text: msg}
			case p.RateLimited():
				skipped := struct{}{}
				tc.Skipped = &skipped
				tc.SystemOut = "warning: " + p.WarningCheck() + " " + p.Warning()
			case p.WarningCheck() != "":
				tc.SystemOut = "warning: " + p.WarningCheck() + " " + p.Warning()
			}
			suite.Cases = append(suite.Cases, tc)
		}
	}

	report := jUnitTestSuites{
		Name:       result.SpecTitle,
		Tests:      suite.Tests,
		Failures:   suite.Failures,
		Errors:     suite.Errors,
		Time:       suite.Time,
		TestSuites: []jUnitSuite{suite},
	}

	out, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

// DiffJUnit returns a JUnit XML report for a spec diff result.
// Each change is emitted as a test case; breaking changes (ERR level) are
// marked as failures so PR annotation tools can surface them inline.
func DiffJUnit(result *openapi.DiffResult) ([]byte, error) {
	return DiffJUnitOn(result, openapi.FailOnBreaking)
}

// DiffJUnitOn is DiffJUnit with a chosen --fail-on threshold: every change at or above it is a
// failure.
func DiffJUnitOn(result *openapi.DiffResult, on openapi.FailOn) ([]byte, error) {
	breaking := result.FailCount(on)

	var cases []jUnitCase
	if len(result.Changes) == 0 {
		cases = append(cases, jUnitCase{
			Name:      "No breaking changes",
			ClassName: "openapi.diff",
			Time:      "0.000",
		})
	} else {
		for _, c := range result.Changes {
			name := fmt.Sprintf("[%s] %s", c.Level, c.ID)
			if c.Method != "" && c.Path != "" {
				name = fmt.Sprintf("[%s] %s  %s %s", c.Level, c.ID, c.Method, c.Path)
			}
			detail := c.Text
			if c.Method != "" && c.Path != "" {
				detail = fmt.Sprintf("%s  %s %s", c.Text, c.Method, c.Path)
			}
			tc := jUnitCase{
				Name:      name,
				ClassName: "openapi.diff",
				Time:      "0.000",
			}
			if c.Fails(on) {
				tc.Failure = &jUnitFailure{Message: detail, Text: detail}
			}
			cases = append(cases, tc)
		}
	}

	suite := jUnitSuite{
		Name:     "API Spec Diff",
		Tests:    len(cases),
		Failures: breaking,
		Time:     "0.000",
		Cases:    cases,
	}
	report := jUnitTestSuites{
		Name:       "API Spec Diff",
		Tests:      suite.Tests,
		Failures:   suite.Failures,
		Time:       suite.Time,
		TestSuites: []jUnitSuite{suite},
	}

	out, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
