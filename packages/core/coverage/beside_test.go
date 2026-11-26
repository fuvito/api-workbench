package coverage

import (
	"path/filepath"
	"testing"
)

// B-152: a workspace in a folder of a larger project, which has a spec of its own at the path the
// workspace's scenarios write.
func projectWithASpecBesideTheWorkspace(t *testing.T) (ws string) {
	t.Helper()
	project := t.TempDir()
	ws = filepath.Join(project, "workspace")
	put(t, filepath.Join(project, "specs", "api.yaml"), "the project's spec")
	put(t, filepath.Join(ws, "specs", "api.yaml"), "the workspace's spec")
	put(t, filepath.Join(ws, "scenarios", "s.yaml"), scenarioNaming("../specs/api.yaml"))
	return ws
}

func TestAnOldEntryIsNotMovedToAFileBesideTheWorkspace(t *testing.T) {
	ws := projectWithASpecBesideTheWorkspace(t)
	putOld(t, ws, "../specs/api.yaml", 4)
	r := ListAll(ws)
	if len(r) != 1 || r[0].SpecFile != "specs/api.yaml" || r[0].TotalRuns != 4 {
		t.Fatalf("the scenario means the workspace's spec: %+v", r)
	}
}

// The app asks with a scenario's specFile as it is written, without saying from which folder.
func TestAScenarioSpellingIsReadFromTheScenarioFolder(t *testing.T) {
	ws := projectWithASpecBesideTheWorkspace(t)
	if got := Identify(ws, "../specs/api.yaml"); got != "specs/api.yaml" {
		t.Errorf("%q", got)
	}
}

// No scenario writes it, so nothing says what folder it was relative to: it is not guessed.
func TestAPathLeavingTheWorkspaceIsNotReadFromItsRoot(t *testing.T) {
	ws := projectWithASpecBesideTheWorkspace(t)
	old := putOld(t, ws, "../specs/other.yaml", 2)
	put(t, filepath.Join(filepath.Dir(ws), "specs", "other.yaml"), "x")
	if r := ListAll(ws); len(r) != 1 || r[0].SpecFile != "../specs/other.yaml" || !exists(old) {
		t.Errorf("kept as it was: %+v", r)
	}
	// a caller that knows its folder can still place it
	if cov := Load(ws, "../specs/other.yaml", ws); cov.TotalRuns != 2 || exists(old) {
		t.Errorf("%+v", cov)
	}
}

// An old entry holds the text a scenario wrote, so the scenario's folder decides, even when the same
// text also names a file from the workspace folder.
func TestAnOldEntryMeansWhatTheScenarioThatWroteItMeant(t *testing.T) {
	ws := t.TempDir()
	put(t, filepath.Join(ws, "api.yaml"), "root")
	put(t, filepath.Join(ws, "scenarios", "api.yaml"), "beside the scenario")
	put(t, filepath.Join(ws, "scenarios", "s.yaml"), scenarioNaming("api.yaml"))
	putOld(t, ws, "api.yaml", 3)
	if r := ListAll(ws); len(r) != 1 || r[0].SpecFile != "scenarios/api.yaml" {
		t.Errorf("%+v", r)
	}
	// while a name as it is stored now is a path from the workspace folder
	if got := Identify(ws, "api.yaml"); got != "api.yaml" {
		t.Errorf("a stored name: %q", got)
	}
}
