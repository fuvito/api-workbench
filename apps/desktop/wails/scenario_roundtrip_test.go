package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fuvito/api-workbench/packages/core/serialization"
)

// The visual scenario editor saves a file it rebuilds from what Go gave it, in TypeScript. These
// files tie the two halves together: this test holds what Go reads (detail.json) and that what the
// editor writes (saved.yaml) is the same scenario; scenario-roundtrip.test.ts holds that the editor
// writes saved.yaml from detail.json.
func TestSavingFromTheScenarioEditorKeepsTheFile(t *testing.T) {
	dir := filepath.Join("..", "testdata", "scenario-roundtrip")
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return []byte(strings.ReplaceAll(string(data), "\r\n", "\n"))
	}

	detail := NewScenarioService(newWS(t, t.TempDir()), NewSettingsService()).LoadScenario(filepath.Join(dir, "original.yaml"))
	if detail.Error != "" {
		t.Fatal(detail.Error)
	}
	detail.Path = ""
	got, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_ROUNDTRIP") != "" {
		if err := os.WriteFile(filepath.Join(dir, "detail.json"), append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if strings.TrimSpace(string(read("detail.json"))) != string(got) {
		t.Errorf("detail.json is not what Go hands the editor (UPDATE_ROUNDTRIP=1 rewrites it):\n%s", got)
	}

	normal := func(name string) string {
		sc, err := serialization.ParseScenario(read(name), "yaml")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out, err := serialization.WriteScenario(sc, "yaml")
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	if want, got := normal("original.yaml"), normal("saved.yaml"); got != want {
		t.Errorf("what the editor saves is not the scenario it loaded:\n--- loaded\n%s\n--- saved\n%s", want, got)
	}
}

func TestSavingARequestAndAnEnvironmentKeepsTheirOwnKeys(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	ws := newWS(t, root)
	write := func(name, body string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	contains := func(path string, keys ...string) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range keys {
			if !strings.Contains(string(data), k+": kept") {
				t.Errorf("%s was lost:\n%s", k, data)
			}
		}
	}
	// the editor gets the file as JSON and sends it back the same way
	through := func(in, out any) {
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}

	reqKeys := []string{"x-top", "x-header", "x-query", "x-body", "x-auth"}
	reqPath := write("one.yaml", `title: one
method: POST
url: http://x
x-top: kept
headers: [{name: A, value: b, x-header: kept}]
queryParams: [{name: q, value: "1", x-query: kept}]
body: {type: json, content: "{}", x-body: kept}
auth: {type: bearer, token: t, x-auth: kept}
`)
	loaded, err := ws.LoadRequest(reqPath)
	if err != nil {
		t.Fatal(err)
	}
	var back RequestFileIO
	through(loaded, &back)
	saved, err := ws.SaveRequest(back)
	if err != nil {
		t.Fatal(err)
	}
	contains(saved, reqKeys...)
	renamed, err := ws.RenameRequest(saved, "two")
	if err != nil {
		t.Fatal(err)
	}
	contains(renamed, reqKeys...)

	envText := "name: e\nx-top: kept\nvariables: [{name: a, value: b, x-var: kept}]\n"
	envPath := write("e.yaml", envText)
	env, err := ws.GetEnvironment(envPath)
	if err != nil {
		t.Fatal(err)
	}
	var envBack EnvironmentIO
	through(env, &envBack)
	if _, err := ws.SaveEnvironment(envBack); err != nil {
		t.Fatal(err)
	}
	contains(envPath, "x-top", "x-var")

	form, err := ws.ParseEnvironmentYAML(envText)
	if err != nil {
		t.Fatal(err)
	}
	if form.Extensions == "" || form.Variables[0].Extensions == "" {
		t.Errorf("the form does not get the keys: %+v", form)
	}
	if _, err := ws.ParseEnvironmentYAML("variables: []\n"); err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("an environment without a name: %v", err)
	}
}
