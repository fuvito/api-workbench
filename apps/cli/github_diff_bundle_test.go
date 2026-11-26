package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inFolder(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

func TestDiffGitHubAnnotatesTheNewSpec(t *testing.T) {
	const old = "openapi: \"3.0.3\"\ninfo: {title: d, version: \"1\"}\npaths:\n  /a:\n    get:\n      responses: {\"200\": {description: ok}}\n  /gone:\n    get:\n      responses: {\"200\": {description: ok}}\n"
	const revised = "openapi: \"3.0.3\"\ninfo: {title: d, version: \"1\"}\npaths:\n  /a:\n    get:\n      parameters: [{name: q, in: query, required: true, schema: {type: string}}]\n      responses: {\"200\": {description: ok}}\n"
	inFolder(t, map[string]string{"old.yaml": old, "new.yaml": revised})

	out, code := runGuide(t, "diff", "old.yaml", "new.yaml", "--format", "github")
	if code != 1 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"::error file=new.yaml,line=5,title=", "GET /a::", // on the operation's line
		"::error file=new.yaml,title=", "GET /gone::", // removed: the file only
		"::notice title=Clear Signal diff::", "at or above --fail-on breaking",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if out, code := runGuide(t, "diff", "old.yaml", "old.yaml", "--format", "github"); code != 0 || strings.Contains(out, "::error") {
		t.Errorf("no change: exit %d\n%s", code, out)
	}
}

func TestBundleRunGitHubPointsAtTheStepInTheBundle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	t.Cleanup(srv.Close)
	step := func(label string) string {
		return "      - label: " + label + "\n        request: {method: GET, url: \"" + srv.URL + "\"}\n        assertions: [{type: status, operator: eq, expected: 200}]\n"
	}
	inFolder(t, map[string]string{"b.yaml": "schemaVersion: \"1\"\ntitle: b\nscenarios:\n  - file: one\n    title: One\n    steps:\n" + step("first") +
		"  - file: two\n    title: Two\n    steps:\n" + step("second")})

	out, code := runGuide(t, "bundle", "run", "b.yaml", "--format", "github")
	if code != 1 || !strings.Contains(out, "::error file=b.yaml,line=7,title=One%3A first::") || !strings.Contains(out, "::error file=b.yaml,line=13,title=Two%3A second::") {
		t.Errorf("exit %d\n%s", code, out)
	}
	// a filter keeps each scenario on its own lines
	out, _ = runGuide(t, "bundle", "run", "b.yaml", "--scenario", "Two", "--format", "github")
	if !strings.Contains(out, "line=13,title=Two%3A second") || strings.Contains(out, "line=7") {
		t.Errorf("filtered:\n%s", out)
	}
	if out, code := runGuide(t, "bundle", "run", "b.yaml", "--report", "github=x.txt"); code != 2 {
		t.Errorf("github is for the screen only: exit %d\n%s", code, out)
	}
}
