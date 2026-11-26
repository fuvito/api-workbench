package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The request editor and Run folder follow the command line: a request that would reach the server
// with {{braces}} in it is not sent.
func TestDesktopRequest_IsNotSentWithAValueNothingDefines(t *testing.T) {
	svc, _ := newRS(t)
	var sent int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sent++ }))
	t.Cleanup(srv.Close)

	for name, in := range map[string]RequestInput{
		"address": {Method: "GET", URL: srv.URL + "/?t={{nowhere}}"},
		"query":   {Method: "GET", URL: srv.URL, QueryParams: []QueryParamIO{{Name: "t", Value: "{{nowhere}}"}}},
		"header":  {Method: "GET", URL: srv.URL, Headers: []HeaderIO{{Name: "X-T", Value: "{{nowhere}}"}}},
		"body":    {Method: "POST", URL: srv.URL, Body: &BodyIO{Type: "text", Content: "{{nowhere}}"}},
		"token":   {Method: "GET", URL: srv.URL, Auth: &AuthIO{Type: "bearer", Token: "{{nowhere}}"}},
	} {
		out := svc.Execute(in)
		if !strings.HasPrefix(out.Error, "not sent: no value for {{nowhere}}") || !strings.Contains(out.Error, "status bar") {
			t.Errorf("%s: %q", name, out.Error)
		}
	}
	if out := svc.Execute(RequestInput{Method: "GET", URL: srv.URL + "/{{$noSuchFunction}}"}); !strings.HasPrefix(out.Error, "not sent: ") {
		t.Errorf("a value that cannot be made: %q", out.Error)
	}
	if sent != 0 {
		t.Fatalf("%d request(s) reached the server", sent)
	}
	if len(svc.GetHistory()) != 0 {
		t.Error("a request that was not sent is not history")
	}

	// what is not chosen is not looked at, and a function still works
	ok := RequestInput{Method: "GET", URL: srv.URL + "/{{$guid}}", Auth: &AuthIO{Type: "none", Token: "{{nowhere}}"}}
	if out := svc.Execute(ok); out.Error != "" || sent != 1 {
		t.Errorf("%q, %d sent", out.Error, sent)
	}
}

func TestDesktopRunFolder_DoesNotSendAnUndefinedValue(t *testing.T) {
	ss, ws := newSS(t)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { paths = append(paths, r.URL.Path) }))
	t.Cleanup(srv.Close)
	folder := filepath.Join(ws.workspacePath, "requests", "f")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, url := range map[string]string{"a.yaml": srv.URL + "/{{nowhere}}", "b.yaml": srv.URL + "/ok"} {
		writeFile(t, filepath.Join(folder, name), "schemaVersion: \"1\"\ntitle: "+name+"\nmethod: GET\nurl: \""+url+"\"\n")
	}
	results := ss.RunFolder(folder)
	if len(results) != 2 || !strings.HasPrefix(results[0].Error, "not sent: no value for {{nowhere}}") || results[0].Passed || !results[1].Passed {
		t.Errorf("%+v", results)
	}
	if len(paths) != 1 || paths[0] != "/ok" {
		t.Errorf("the server received %v", paths)
	}
}
