package serialization

import (
	"strings"
	"testing"

	"github.com/fuvito/api-workbench/packages/core/domain"
)

// one x- key at every level a scenario has; each value names where it was
const scenarioWithOwnKeys = `schemaVersion: "1"
x-top: top
title: s
variables:
  - {name: a, value: b, x-var: var}
steps:
  - label: one
    x-step: step
    request:
      x-request: request
      method: POST
      url: http://x
      headers: [{name: A, value: b, x-header: header}]
      queryParams: [{name: q, value: "1", x-query: query}]
      body: {type: json, content: "{}", x-body: body}
      auth: {type: bearer, token: t, x-auth: auth}
    retry: {maxAttempts: 2, x-retry: retry}
    assertions: [{type: status, expected: 200, x-assertion: assertion}]
    extractions: [{name: n, source: status, x-extraction: extraction}]
`

var scenarioLevels = []string{"top", "var", "step", "request", "header", "query", "body", "auth", "retry", "assertion", "extraction"}

func TestEveryOwnKeyOfAScenarioIsWrittenBack(t *testing.T) {
	sc, err := ParseScenarioFile([]byte(scenarioWithOwnKeys), "s.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"yaml", "json"} {
		out, err := WriteScenario(sc, format)
		if err != nil {
			t.Fatal(err)
		}
		for _, level := range scenarioLevels {
			if !strings.Contains(string(out), "x-"+level) {
				t.Errorf("%s: x-%s was lost:\n%s", format, level, out)
			}
		}
		again, err := ParseScenario(out, format)
		if err != nil {
			t.Fatalf("%s: what was written does not read back: %v\n%s", format, err, out)
		}
		// compared as text: JSON reads numbers as another type, which writes the same
		want, _ := WriteScenario(sc, "yaml")
		if got, _ := WriteScenario(again, "yaml"); string(got) != string(want) {
			t.Errorf("%s: not the same scenario after a round trip:\n%s\n%s", format, want, got)
		}
	}
}

func TestAnOwnKeyStaysWithItsItemWhenItemsMove(t *testing.T) {
	sc, err := ParseScenarioFile([]byte("title: s\nsteps:\n  - {label: a, x-mine: A, requestRef: a.yaml}\n  - {label: b, x-mine: B, requestRef: b.yaml}\n"), "s.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sc.Steps[0], sc.Steps[1] = sc.Steps[1], sc.Steps[0]
	sc.Steps[0].Label = "renamed"
	out, err := WriteScenario(sc, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseScenario(out, "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Steps[0].Extensions; got != `"x-mine": "B"` {
		t.Errorf("the first step has %s\n%s", got, out)
	}
}

func TestOwnKeyValuesKeepTheirTypeAndOrder(t *testing.T) {
	body := "method: GET\nurl: http://x\n" +
		"x-z: 12345678901234567890\n" +
		"x-a: {b: [1, 2.5, true, null], c: \"true\", d: \"007\", e: \"two\\nlines\"}\n" +
		"x-text: \"a: b # c <&> 😀\"\n" +
		"x-hex: 0x1F\n" +
		"x-base: &base [1]\n" +
		"x-again: *base\n"
	req, err := ParseRequestFile([]byte(body), "r.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Extensions(strings.Join([]string{
		`"x-z": 12345678901234567890`,
		`"x-a": {"b":[1,2.5,true,null],"c":"true","d":"007","e":"two\nlines"}`,
		`"x-text": "a: b # c <&> 😀"`,
		`"x-hex": 31`,
		`"x-base": [1]`,
		`"x-again": [1]`,
	}, "\n"))
	if req.Extensions != want {
		t.Fatalf("read:\n%s\nwant:\n%s", req.Extensions, want)
	}
	for _, format := range []string{"yaml", "json"} {
		out, err := WriteRequest(req, format)
		if err != nil {
			t.Fatal(err)
		}
		again, err := ParseRequest(out, format)
		if err != nil {
			t.Fatalf("%s: %v\n%s", format, err, out)
		}
		if again.Extensions != want {
			t.Errorf("%s: after a round trip:\n%s\n%s", format, again.Extensions, out)
		}
	}
}

func TestEnvironmentOwnKeysAreWrittenBackInJSONAndYAML(t *testing.T) {
	// the escaped surrogate pair is JSON the YAML parser refuses
	in := `{"schemaVersion":"1","name":"e","x-owner":"😀","variables":[{"name":"a","value":"b","x-note":{}}]}`
	env, err := ParseEnvironment([]byte(in), "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"yaml", "json"} {
		out, err := WriteEnvironment(env, format)
		if err != nil {
			t.Fatal(err)
		}
		again, err := ParseEnvironment(out, format)
		if err != nil {
			t.Fatalf("%s: %v\n%s", format, err, out)
		}
		if again.Extensions != "\"x-owner\": \"😀\"" || again.Variables[0].Extensions != `"x-note": {}` {
			t.Errorf("%s: %q %q\n%s", format, again.Extensions, again.Variables[0].Extensions, out)
		}
	}
}

func TestOnlyOwnKeysCanBeWrittenThroughExtensions(t *testing.T) {
	for _, bad := range []domain.Extensions{`"url": "http://elsewhere"`, `"x-a": 1, "url": "y"`, `x-a: 1`} {
		req := &domain.Request{Method: "GET", URL: "http://x", Extensions: bad}
		if out, err := WriteRequest(req, "yaml"); err == nil {
			t.Errorf("%s was written:\n%s", bad, out)
		}
	}
}

func TestABundleDoesNotCarryOwnKeys(t *testing.T) {
	sc, err := ParseScenarioFile([]byte(scenarioWithOwnKeys), "s.yaml")
	if err != nil {
		t.Fatal(err)
	}
	b := &Bundle{
		Requests:     []BundleRequest{{File: "r", Request: sc.Steps[0].Request}},
		Scenarios:    []BundleScenario{{Scenario: sc}},
		Environments: []BundleEnvironment{{Environment: &domain.Environment{Name: "e", Extensions: `"x-env": 1`}}},
	}
	for _, format := range []string{"yaml", "json"} {
		out, err := WriteBundle(b, format)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "x-") {
			t.Errorf("%s: a bundle is for sharing and must not carry x- keys:\n%s", format, out)
		}
		if _, err := ParseBundle(out, format); err != nil {
			t.Errorf("%s: the bundle does not import: %v", format, err)
		}
	}
}
