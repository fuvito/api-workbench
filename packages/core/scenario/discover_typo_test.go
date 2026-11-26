package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

// A folder run must not leave out a scenario because "steps" is misspelled: it would not run, and
// nothing would fail.
func TestDiscoverKeepsAScenarioWithMisspelledSteps(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"a.yaml":    "title: a\nstpes: []\n",
		"b.yaml":    "title: b\nSteps: []\n",
		"c.yaml":    "title: c\nstep: []\n",
		"good.yaml": "title: g\nsteps: []\n",
		// not scenarios
		"request.yaml": "title: r\nmethod: GET\nurl: http://x\n",
		"env.yaml":     "name: e\nvariables: []\n",
		"spec.yaml":    "openapi: 3.0.3\ninfo: {title: s, version: '1'}\npaths: {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found, err := Discover([]string{dir}, true)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range found.Files {
		names = append(names, filepath.Base(f))
	}
	if len(names) != 4 || len(found.Skipped) != 3 {
		t.Errorf("run %v, skipped %v", names, found.Skipped)
	}
}
