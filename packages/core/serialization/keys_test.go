package serialization

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scenarioError(t *testing.T, body, name string) string {
	t.Helper()
	_, err := ParseScenarioFile([]byte(body), name)
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestAMisspelledKeyIsRefusedWithTheSpellingMeant(t *testing.T) {
	step := "title: s\nsteps:\n  - label: a\n    request: {method: GET, url: http://x}\n"
	for body, want := range map[string][]string{
		step + "    asertions: [{type: status, expected: 200}]\n": {`unknown key "asertions" in steps[0]`, `did you mean "assertions"?`},
		step + "    name: a\n":                                                                      {`unknown key "name" in steps[0]`, `did you mean "label"?`},
		step + "    failpolicy: stop\n":                                                             {`did you mean "failPolicy"?`},
		step + "    assertions: [{type: status, expect: 200}]\n":                                    {`unknown key "expect" in steps[0].assertions[0]`, `did you mean "expected"?`},
		strings.Replace(step, "url: http://x}", "url: http://x, timeoutMs: 5}", 1):                  {`unknown key "timeoutMs" in steps[0].request`, `did you mean "timeout"?`},
		strings.Replace(step, "url: http://x}", "url: http://x, headers: [{key: A, value: b}]}", 1): {`unknown key "key" in steps[0].request.headers[0]`},
		"specfile: a.yaml\n" + step:                                                                 {`unknown key "specfile"`, `did you mean "specFile"?`},
		step + "    extract: [{name: a, source: status}]\n":                                         {`did you mean "extractions"?`},
		step + "    extractions: [{name: a, from: status}]\n":                                       {`did you mean "source"?`},
		step + "    somethingElse: 1\n":                                                             {`unknown key "somethingElse" in steps[0]`},
	} {
		got := scenarioError(t, body, "s.yaml")
		for _, w := range append(want, "Keys that start with x-") {
			if !strings.Contains(got, w) {
				t.Errorf("missing %q in %q\n%s", w, got, body)
			}
		}
	}
	// JSON is held to the exact spelling too: the decoder would match it in any case
	if got := scenarioError(t, `{"Title": "s", "steps": []}`, "s.json"); !strings.Contains(got, `did you mean "title"?`) {
		t.Errorf("JSON: %q", got)
	}
}

func TestRequestAndEnvironmentFilesAreHeldToTheirKeys(t *testing.T) {
	if _, err := ParseRequestFile([]byte("method: GET\nurl: http://x\nfollowRedirect: false\n"), "r.yaml"); err == nil || !strings.Contains(err.Error(), `did you mean "followRedirects"?`) {
		t.Errorf("request: %v", err)
	}
	if _, err := ParseEnvironmentFile([]byte("name: e\nvariables: [{name: a, value: b, secrte: true}]\n"), "e.yaml"); err == nil || !strings.Contains(err.Error(), `unknown key "secrte" in variables[0] (did you mean "secret"?)`) {
		t.Errorf("environment: %v", err)
	}
}

func TestOwnKeysAndKeysOfOldReleasesAreIgnored(t *testing.T) {
	body := `schemaVersion: "1"
x-owner: team
title: s
variables: [{name: a, value: b, x-note: y}]
steps:
  - label: a
    x-ticket: ABC-1
    title: from 0.3.0
    description: from 0.3.0
    request: {method: GET, url: http://x, description: from 0.1.0, x-a: {anything: [1, 2]}}
    assertions: [{type: jsonpath, path: $.a, operator: eq, expected: {any: {keys: true}}}]
`
	if got := scenarioError(t, body, "s.yaml"); got != "" {
		t.Error(got)
	}
	// anchors and merge keys keep working
	merged := "title: s\nx-defaults: &d {label: a}\nsteps:\n  - <<: *d\n    request: {method: GET, url: http://x}\n"
	if got := scenarioError(t, merged, "s.yaml"); got != "" {
		t.Error(got)
	}
}

// A line missing its space makes the value part of the key. The error must not repeat it.
func TestTheErrorNeverRepeatsAValue(t *testing.T) {
	for _, body := range []string{
		"name: e\nvariables: [{name: a, value: b, password:hunter2}]\n",
		"name: e\nvariables: [{name: a, value: b, \"token=hunter2\": }]\n",
		"name: e\nvariables: [{name: a, value: b, \"Authorization Bearer hunter2\": 1}]\n",
	} {
		_, err := ParseEnvironmentFile([]byte(body), "e.yaml")
		if err == nil || strings.Contains(err.Error(), "hunter2") {
			t.Errorf("%q -> %v", body, err)
		}
	}
}

func TestANewerVersionIsNamedBeforeItsNewKeys(t *testing.T) {
	got := scenarioError(t, "schemaVersion: \"9\"\ntitle: s\nsomethingNew: 1\nsteps: []\n", "s.yaml")
	if !strings.Contains(got, `schemaVersion "9" is not supported`) || strings.Contains(got, "somethingNew") {
		t.Error(got)
	}
}

// Every scenario, request and environment file the repository ships must be read.
func TestShippedFilesHaveNoUnknownKeys(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	checked := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if n := d.Name(); n == "node_modules" || n == "dist" || n == "build" || (strings.HasPrefix(n, ".") && n != ".." && n != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		slashed := filepath.ToSlash(path)
		var parse func([]byte, string) error
		switch {
		case filepath.Ext(path) != ".yaml":
		case strings.Contains(slashed, "/scenarios/"):
			parse = func(b []byte, n string) error { _, err := ParseScenarioFile(b, n); return err }
		case strings.Contains(slashed, "/environments/"):
			parse = func(b []byte, n string) error { _, err := ParseEnvironmentFile(b, n); return err }
		case strings.Contains(slashed, "/requests/"):
			parse = func(b []byte, n string) error { _, err := ParseRequestFile(b, n); return err }
		}
		if parse == nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		checked++
		if err := parse(data, d.Name()); err != nil && strings.Contains(err.Error(), "unknown key") {
			t.Errorf("%s: %v", slashed, err)
		}
		return nil
	})
	if checked < 20 {
		t.Fatalf("only %d files were found, so little is checked", checked)
	}
}
