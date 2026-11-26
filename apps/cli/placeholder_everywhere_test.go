package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// After clrsig bundle import the environment file holds the placeholder: no command sends it.
func TestThePlaceholderIsNotSentByAnyCommand(t *testing.T) {
	var headers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = append(headers, r.Header.Get("Authorization")+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	env := writeTemp(t, "e.yaml", "schemaVersion: \"1\"\nname: e\nvariables: [{name: token, value: CHANGE_ME, secret: true}]\n")
	request := writeTemp(t, "r.yaml", "schemaVersion: \"1\"\ntitle: r\nmethod: GET\nurl: \""+srv.URL+"\"\nauth: {type: bearer, token: \"{{token}}\"}\n")
	scenarioFile := writeTemp(t, "s.yaml", scenarioWith("    request: {method: GET, url: \""+srv.URL+"\", auth: {type: bearer, token: \"{{token}}\"}}\n"))
	spec := writeTemp(t, "spec.yaml", "openapi: \"3.0.3\"\ninfo: {title: p, version: \"1\"}\npaths:\n  /a:\n    get:\n      parameters: [{name: token, in: query, required: true, schema: {type: string}}]\n      responses: {\"200\": {description: ok}}\n")

	for name, args := range map[string][]string{
		"run":     {"run", scenarioFile, "-e", env},
		"request": {"request", request, "-e", env},
		"conform": {"conform", spec, "-b", srv.URL, "-e", env},
	} {
		out, code := runGuide(t, args...)
		if code != 1 || !strings.Contains(out, `"token" still holds the CHANGE_ME`) {
			t.Errorf("%s: exit %d\n%s", name, code, out)
		}
	}
	for _, h := range headers {
		if strings.Contains(h, "CHANGE_ME") {
			t.Errorf("the server received %q", h)
		}
	}
}
