package coverage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scenarioNaming(specFile string) string {
	return "schemaVersion: \"1\"\ntitle: s\nspecFile: " + specFile + "\nsteps:\n  - label: a\n    request: {method: GET, url: http://x}\n"
}

// putOld writes an entry the way releases up to 0.13.5 did.
func putOld(t *testing.T, ws, written string, runs int) string {
	t.Helper()
	data, err := json.Marshal(StoredCoverage{SpecFile: written, TotalRuns: runs, TotalOps: 3, Exercised: []string{"GET /a"}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(DirPath(ws), legacyKey(written))
	put(t, path, string(data))
	return path
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestOneFileSpelledTwoWaysIsOneEntry(t *testing.T) {
	ws := t.TempDir()
	put(t, filepath.Join(ws, "specs", "api.yaml"), "x")
	for _, spelled := range []struct{ ref, base string }{
		{"../specs/api.yaml", filepath.Join(ws, "scenarios")},
		{"../../specs/api.yaml", filepath.Join(ws, "scenarios", "deep")},
		{"specs/api.yaml", ""},
		{filepath.Join(ws, "specs", "api.yaml"), ""},
		{"api.yaml", filepath.Join(ws, "specs")},
	} {
		if got := Identify(ws, spelled.ref, spelled.base); got != "specs/api.yaml" {
			t.Errorf("%q from %q is %q", spelled.ref, spelled.base, got)
		}
	}
	if runtime.GOOS == "windows" {
		if got := Identify(ws, "SPECS/API.YAML"); got != "specs/api.yaml" {
			t.Errorf("another case on Windows: %q", got)
		}
	}
}

func TestADeletedSpecKeepsItsEntry(t *testing.T) {
	ws := t.TempDir()
	spec := filepath.Join(ws, "specs", "api.yaml")
	put(t, spec, "x")
	if err := Save(ws, StoredCoverage{SpecFile: spec, TotalRuns: 2}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(spec); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{spec, "specs/api.yaml"} {
		if got := Load(ws, ref); got.TotalRuns != 2 || got.SpecFile != "specs/api.yaml" {
			t.Errorf("%q: %+v", ref, got)
		}
	}
}

func TestDifferentSpecsAreDifferentEntries(t *testing.T) {
	ws := t.TempDir()
	// written alike, and under the old key also stored alike
	for _, dir := range []string{"a", "b"} {
		put(t, filepath.Join(ws, dir, "openapi.yaml"), "x")
		cov := Load(ws, "openapi.yaml", filepath.Join(ws, dir))
		cov.TotalRuns++
		if err := Save(ws, cov); err != nil {
			t.Fatal(err)
		}
	}
	put(t, filepath.Join(ws, "my_spec.yaml"), "x")
	put(t, filepath.Join(ws, "my.spec.yaml"), "x")
	for _, ref := range []string{"my_spec.yaml", "my.spec.yaml", "https://h/spec?a=1", "https://h/spec/a/1"} {
		cov := Load(ws, ref)
		cov.TotalRuns++
		if err := Save(ws, cov); err != nil {
			t.Fatal(err)
		}
	}
	var names []string
	for _, r := range ListAll(ws) {
		if r.TotalRuns != 1 {
			t.Errorf("%s has %d runs", r.SpecFile, r.TotalRuns)
		}
		names = append(names, r.SpecFile)
	}
	if len(names) != 6 {
		t.Errorf("want six entries, got %v", names)
	}
}

func TestAnOutsideFileKeepsItsFullPath(t *testing.T) {
	ws, other := t.TempDir(), t.TempDir()
	put(t, filepath.Join(other, "api.yaml"), "x")
	got := Identify(ws, filepath.Join(other, "api.yaml"))
	if !strings.HasSuffix(got, "/api.yaml") || !filepath.IsAbs(filepath.FromSlash(got)) || Identify(ws, got) != got {
		t.Errorf("%q", got)
	}
}

func TestOldEntriesAreRenamedOnFirstRead(t *testing.T) {
	ws := t.TempDir()
	put(t, filepath.Join(ws, "specs", "api.yaml"), "x")
	put(t, filepath.Join(ws, "scenarios", "s.yaml"), scenarioNaming("../specs/api.yaml"))
	oldFile := putOld(t, ws, "../specs/api.yaml", 4)
	oldURL := putOld(t, ws, "https://h/openapi.yaml", 2)
	oldLost := putOld(t, ws, "../gone/api.yaml", 7)

	got := map[string]int{}
	for _, r := range ListAll(ws) {
		got[r.SpecFile] = r.TotalRuns
	}
	want := map[string]int{"specs/api.yaml": 4, "https://h/openapi.yaml": 2, "../gone/api.yaml": 7}
	for name, runs := range want {
		if got[name] != runs {
			t.Errorf("%s: %d runs, want %d (all: %v)", name, got[name], runs, got)
		}
	}
	if len(got) != len(want) || exists(oldFile) || exists(oldURL) {
		t.Errorf("renamed, not copied: %v", got)
	}
	if !exists(oldLost) {
		t.Error("an entry that cannot be matched is kept")
	}
	// and it can still be read and reset by the name it has
	if Load(ws, "../gone/api.yaml").TotalRuns != 7 {
		t.Error("the unmatched entry cannot be read")
	}
	if err := Reset(ws, "../gone/api.yaml"); err != nil || exists(oldLost) {
		t.Errorf("reset: %v", err)
	}
}

func TestAnOldEntryOnlyTheCallersFolderCanPlaceIsRenamedThen(t *testing.T) {
	ws, project := t.TempDir(), t.TempDir()
	put(t, filepath.Join(project, "openapi.yaml"), "x")
	old := putOld(t, ws, "openapi.yaml", 3) // recorded with --spec from the project folder
	if ListAll(ws); !exists(old) {
		t.Fatal("the workspace alone cannot place it, so it stays")
	}
	cov := Load(ws, "openapi.yaml", project)
	if cov.TotalRuns != 3 || cov.SpecFile != Identify(ws, "openapi.yaml", project) || exists(old) {
		t.Errorf("%+v", cov)
	}
}

func TestAnAmbiguousOldEntryIsLeftAlone(t *testing.T) {
	ws := t.TempDir()
	for _, dir := range []string{"one", "two"} {
		put(t, filepath.Join(ws, "scenarios", dir, "spec.yaml"), "x")
		put(t, filepath.Join(ws, "scenarios", dir, "s.yaml"), scenarioNaming("spec.yaml"))
	}
	old := putOld(t, ws, "spec.yaml", 5)
	if r := ListAll(ws); len(r) != 1 || r[0].SpecFile != "spec.yaml" || !exists(old) {
		t.Errorf("two files are meant, so neither is guessed: %+v", r)
	}
}

func TestRenamingNeverReplacesANewerEntry(t *testing.T) {
	ws := t.TempDir()
	put(t, filepath.Join(ws, "api.yaml"), "x")
	if err := Save(ws, StoredCoverage{SpecFile: "api.yaml", TotalRuns: 9}); err != nil {
		t.Fatal(err)
	}
	old := putOld(t, ws, "api.yaml", 1) // an older release ran again afterwards
	if got := Load(ws, "api.yaml"); got.TotalRuns != 9 || !exists(old) {
		t.Errorf("%+v", got)
	}
}
