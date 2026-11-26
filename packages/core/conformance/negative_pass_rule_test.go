package conformance_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fuvito/api-workbench/packages/core/application"
	"github.com/fuvito/api-workbench/packages/core/checks"
	"github.com/fuvito/api-workbench/packages/core/conformance"
	"github.com/fuvito/api-workbench/packages/core/executor"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/reporting"
)

func TestNegativeProbe_OutcomeByStatus(t *testing.T) {
	expected := map[openapi.ProbeKind][]int{
		openapi.ProbeDropRequiredField:    {400, 422},
		openapi.ProbeWrongFieldType:       {400, 422},
		openapi.ProbeMissingRequiredParam: {400, 422, 428},
		openapi.ProbeMissingAuth:          {401, 403},
	}
	for kind, fine := range expected {
		for _, status := range []int{200, 201, 302, 400, 401, 403, 404, 405, 409, 415, 422, 428, 429, 500} {
			p := conformance.NegativeProbeResult{Kind: kind, StatusCode: status}
			isFine := false
			for _, f := range fine {
				isFine = isFine || f == status
			}
			var failed, passed, limited bool
			check := ""
			switch {
			case status == 429:
				limited, check = true, "CS-PROBE-008"
			case status >= 400 && status < 500:
				passed = true
				if !isFine {
					check = "CS-PROBE-007"
				}
			default:
				failed = true
			}
			if p.Failed() != failed || p.Passed() != passed || p.RateLimited() != limited || p.WarningCheck() != check {
				t.Errorf("%s %d: failed=%v passed=%v limited=%v check=%q", kind, status, p.Failed(), p.Passed(), p.RateLimited(), p.WarningCheck())
			}
			if p.FailsRun(checks.Failure) != failed || p.FailsRun(checks.Warning) != (failed || check != "") {
				t.Errorf("%s %d: what fails the run is wrong", kind, status)
			}
		}
	}
	for _, silent := range []conformance.NegativeProbeResult{{Skipped: true}, {Error: "no answer"}} {
		if silent.Failed() || silent.RateLimited() || silent.WarningCheck() != "" {
			t.Errorf("a probe with no answer has no outcome of its own: %+v", silent)
		}
	}
}

const outcomeSpec = `
openapi: "3.0.3"
info: {title: p, version: "1"}
paths:
  /things:
    post:
      requestBody: {required: true, content: {application/json: {schema: {type: object, required: [name], properties: {name: {type: string}}}}}}
      responses: {"201": {description: ok}}
`

// run sends the probes to a server that answers every POST of a bad body with status.
func runProbes(t *testing.T, status int, failOn checks.Severity, baseline *conformance.Baseline) application.ConformOutput {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 512)
		n, _ := r.Body.Read(body)
		if text := string(body[:n]); strings.Contains(text, `"name":"x"`) {
			w.WriteHeader(201)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	spec, err := openapi.ParseBytes([]byte(outcomeSpec), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	return application.RunConformance(context.Background(), application.ConformInput{
		Exec: executor.New(executor.Options{}), Spec: spec, BaseURL: srv.URL, Negative: true, FailOn: failOn, Baseline: baseline,
		OnlyOperations: nil,
	})
}

func probeText(out application.ConformOutput) string { return reporting.NegativeText(out.Negative) }

func negativeOnlyViolations(out application.ConformOutput) (ids []string) {
	for _, v := range out.Violations {
		if v.Kind == "probe" {
			ids = append(ids, v.Check)
		}
	}
	return ids
}

func TestNegativeRun_RefusedForAnotherReasonWarns(t *testing.T) {
	out := runProbes(t, 404, checks.Failure, nil)
	text := probeText(out)
	for _, want := range []string{"WARN  ", "refused with 404, not 400 or 422", "CS-PROBE-007", "PASSED with 2 warnings"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	if got := negativeOnlyViolations(out); len(got) != 0 || out.Negative.FailingProbes() != 0 {
		t.Errorf("a warning does not fail by default: %v", got)
	}
	strict := runProbes(t, 404, checks.Warning, nil)
	if !strict.Failed || strict.Negative.FailingProbes() != 2 || !strings.Contains(probeText(strict), "FAILED") {
		t.Errorf("--fail-on warning: %v\n%s", strict.Failed, probeText(strict))
	}
	if got := negativeOnlyViolations(strict); len(got) != 2 || got[0] != "CS-PROBE-007" {
		t.Errorf("recorded under its own check: %v", got)
	}
}

func TestNegativeRun_ARateLimitLeavesTheProbeUnchecked(t *testing.T) {
	out := runProbes(t, 429, checks.Failure, nil)
	text := probeText(out)
	for _, want := range []string{"not checked: the server answered 429", "CS-PROBE-008", "NOTHING CHECKED  0 probes", "2 not checked"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "PASS  ") || strings.Contains(text, "no negative probe applies") {
		t.Errorf("a rate limit is not a pass, and the probes did apply:\n%s", text)
	}
	if strict := runProbes(t, 429, checks.Warning, nil); !strict.Failed {
		t.Error("--fail-on warning fails when probes could not be checked")
	}
}

// A recorded accepted-bad-request must not be reported as fixed because a rate limit hid it.
func TestNegativeRun_ARateLimitDoesNotFixABaselineViolation(t *testing.T) {
	broken := runProbes(t, 201, checks.Failure, nil)
	recorded := negativeOnlyViolations(broken)
	if len(recorded) != 2 {
		t.Fatalf("%v", broken.Violations)
	}
	var kept []conformance.Violation
	for _, v := range broken.Violations {
		if v.Kind == "probe" {
			kept = append(kept, v)
		}
	}
	base := &conformance.Baseline{Violations: kept}
	limited := runProbes(t, 429, checks.Failure, base)
	for _, v := range limited.Diff.Fixed {
		if v.Kind == "probe" {
			t.Errorf("reported as fixed: %+v", v)
		}
	}
	for _, status := range []int{400, 422} {
		if fixed := runProbes(t, status, checks.Failure, base); len(fixed.Diff.Fixed) != 2 {
			t.Errorf("%d really fixes them: %+v", status, fixed.Diff.Fixed)
		}
	}
}

func TestProbeReportsCarryTheWarnings(t *testing.T) {
	out := runProbes(t, 404, checks.Failure, nil)
	limited := runProbes(t, 429, checks.Failure, nil)
	for name, produce := range map[string]func(application.ConformOutput) string{
		"json": func(o application.ConformOutput) string {
			b, err := reporting.ConformJSON(o.Result, nil, o.Negative)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		},
		"junit": func(o application.ConformOutput) string {
			b, err := reporting.ConformJUnit(o.Result, o.Negative)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		},
		"github": func(o application.ConformOutput) string {
			return reporting.ConformGitHub(o.Result, o.Negative, nil)
		},
	} {
		if got := produce(out); !strings.Contains(got, "CS-PROBE-007") {
			t.Errorf("%s: no CS-PROBE-007:\n%s", name, got)
		}
		if got := produce(limited); !strings.Contains(got, "CS-PROBE-008") {
			t.Errorf("%s: no CS-PROBE-008:\n%s", name, got)
		}
	}
}
