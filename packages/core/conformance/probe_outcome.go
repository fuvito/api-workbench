package conformance

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/fuvito/api-workbench/packages/core/checks"
	"github.com/fuvito/api-workbench/packages/core/openapi"
)

// A probe asks whether the server refuses a request for the one thing wrong with it. Any 4xx says
// it was refused; only some say it was refused for that (CS-GAP-012). A 429 says nothing at all:
// a rate limit answers before the request is read.

// Check IDs of the two answers that are neither a pass nor an accepted bad request.
const (
	CheckProbeOtherReason = "CS-PROBE-007"
	CheckProbeRateLimited = "CS-PROBE-008"
)

// expectedRefusals are the statuses that name the fault a kind of probe introduces. 428 is the
// status for a required precondition header that is missing.
var expectedRefusals = map[openapi.ProbeKind][]int{
	openapi.ProbeDropRequiredField:    {http.StatusBadRequest, http.StatusUnprocessableEntity},
	openapi.ProbeWrongFieldType:       {http.StatusBadRequest, http.StatusUnprocessableEntity},
	openapi.ProbeMissingRequiredParam: {http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusPreconditionRequired},
	openapi.ProbeMissingAuth:          {http.StatusUnauthorized, http.StatusForbidden},
}

func (r NegativeProbeResult) answered() bool { return !r.Skipped && r.Error == "" }

// RateLimited reports an answer that leaves the probe not checked.
func (r NegativeProbeResult) RateLimited() bool {
	return r.answered() && r.StatusCode == http.StatusTooManyRequests
}

// Failed reports a server that did not refuse the bad request.
func (r NegativeProbeResult) Failed() bool {
	return r.answered() && !r.Passed() && !r.RateLimited()
}

// WarningCheck is the check ID of what is doubtful about the answer, or "" when nothing is.
func (r NegativeProbeResult) WarningCheck() string {
	switch {
	case r.RateLimited():
		return CheckProbeRateLimited
	case r.answered() && r.Passed():
		expected, known := expectedRefusals[r.Kind]
		if known && !slices.Contains(expected, r.StatusCode) {
			return CheckProbeOtherReason
		}
	}
	return ""
}

// Warning says in one sentence what is doubtful, or "".
func (r NegativeProbeResult) Warning() string {
	switch r.WarningCheck() {
	case CheckProbeRateLimited:
		return "not checked: the server answered 429 (too many requests), which it can do without reading the request"
	case CheckProbeOtherReason:
		want := expectedRefusals[r.Kind]
		return fmt.Sprintf("refused with %d, not %s: it may have been refused for another reason than the one tested", r.StatusCode, statusList(want))
	}
	return ""
}

// WarningSeverity is the severity of WarningCheck, so that a lowered or raised check is honoured.
func (r NegativeProbeResult) WarningSeverity() checks.Severity {
	if id := r.WarningCheck(); id != "" {
		return checks.SeverityOf(id)
	}
	return ""
}

func statusList(codes []int) string {
	text := ""
	for i, c := range codes {
		switch {
		case i == 0:
		case i == len(codes)-1:
			text += " or "
		default:
			text += ", "
		}
		text += fmt.Sprint(c)
	}
	return text
}

// FailsRun reports a probe that fails the run: the bad request was accepted, or got no answer's
// worth of proof while warnings are asked to fail.
func (r NegativeProbeResult) FailsRun(failOn checks.Severity) bool {
	if r.Failed() {
		return true
	}
	if !r.answered() {
		return false
	}
	severity := r.WarningSeverity()
	return severity == checks.Failure || (severity == checks.Warning && failOn == checks.Warning)
}

// Tally counts what Counts does not: probes refused for what may be another reason (they are among
// the passed ones) and probes left unchecked by a rate limit (they are in none of Counts).
func (r *NegativeResult) Tally() (otherReason, rateLimited int) {
	for _, p := range r.Probes {
		switch p.WarningCheck() {
		case CheckProbeOtherReason:
			otherReason++
		case CheckProbeRateLimited:
			rateLimited++
		}
	}
	return
}

// FailingProbes is how many probes fail the run beyond those that got no answer.
func (r *NegativeResult) FailingProbes() int {
	n := 0
	for _, p := range r.Probes {
		if p.FailsRun(r.FailOn) {
			n++
		}
	}
	return n
}

// WithoutUnchecked drops from the fixed violations those of a probe a rate limit left unchecked:
// nothing says it is fixed.
func WithoutUnchecked(fixed []Violation, neg *NegativeResult) []Violation {
	if neg == nil {
		return fixed
	}
	unchecked := map[string]bool{}
	for _, p := range neg.Probes {
		if p.RateLimited() {
			unchecked[p.Method+" "+p.Path+"\x00"+p.Description] = true
		}
	}
	if len(unchecked) == 0 {
		return fixed
	}
	kept := fixed[:0:0]
	for _, v := range fixed {
		if v.Kind != "probe" || !unchecked[v.Operation+"\x00"+v.Summary] {
			kept = append(kept, v)
		}
	}
	return kept
}
