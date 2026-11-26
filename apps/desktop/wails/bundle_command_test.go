package main

import (
	"strings"
	"testing"
)

func TestBundleImportCommand(t *testing.T) {
	isolateHome(t)
	ws := newWS(t, t.TempDir())
	svc := NewBundleService(ws)

	pasted := svc.BundleImportCommand("", "  ", "rename")
	if !strings.HasPrefix(pasted.Command, "clrsig bundle import bundle.yaml --workspace ") || strings.Contains(pasted.Command, "--on-conflict") ||
		strings.Contains(pasted.Command, "--spec-file") || !strings.Contains(pasted.Note, "Save the bundle's text as bundle.yaml") {
		t.Errorf("pasted: %+v", pasted)
	}
	full := svc.BundleImportCommand("/tmp/my bundle.yaml", "https://h/spec?a=1&b=2", "overwrite")
	for _, want := range []string{"'/tmp/my bundle.yaml'", "--spec-file 'https://h/spec?a=1&b=2'", "--on-conflict overwrite"} {
		if !strings.Contains(full.Command, want) {
			t.Errorf("missing %s in %s", want, full.Command)
		}
	}
	if full.Note != "" {
		t.Errorf("a saved file needs no note: %q", full.Note)
	}
	if got := NewBundleService(newWS(t, "")).BundleImportCommand("", "", "rename"); got.Error == "" {
		t.Errorf("no workspace: %+v", got)
	}
}
