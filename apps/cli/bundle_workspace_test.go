package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBundleRunRecordsCoverage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	scenario := func(file, path, spec string) string {
		return "  - file: " + file + "\n    title: " + file + "\n" + spec + "    steps:\n      - label: a\n        request: {method: GET, url: \"" + srv.URL + path + "\"}\n        assertions: [{type: status, operator: eq, expected: 200}]\n"
	}
	inFolder(t, map[string]string{
		"spec.yaml": generateSpec,
		"b.yaml":    "schemaVersion: \"1\"\ntitle: b\nscenarios:\n" + scenario("one", "/a", "    specFile: spec.yaml\n") + scenario("two", "/b", "    specFile: spec.yaml\n") + scenario("none", "/a", ""),
	})
	out, code := runGuide(t, "bundle", "run", "b.yaml", "--workspace", "ws")
	if code != 0 || !strings.Contains(out, `scenario "none" was not added to coverage`) {
		t.Fatalf("exit %d\n%s", code, out)
	}
	list, _ := runGuide(t, "coverage", "list", "--workspace", "ws")
	if !strings.Contains(list, "Covered 2 (100%)") || !strings.Contains(list, "2 run(s)") || strings.Count(list, "spec.yaml") != 1 {
		t.Errorf("both scenarios on one entry:\n%s", list)
	}
	// without the flag nothing is written
	if out, code := runGuide(t, "bundle", "run", "b.yaml"); code != 0 || isDir(".clrsig-coverage") {
		t.Errorf("exit %d\n%s", code, out)
	}
}
