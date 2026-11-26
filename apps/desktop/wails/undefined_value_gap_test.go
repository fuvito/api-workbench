package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Pins CS-GAP-028: the request editor sends a {{name}} nothing defines as text, unlike the command
// line and unlike scenario runs.
func TestDesktopRequest_SendsAnUndefinedValueAsWritten_KnownGap(t *testing.T) {
	svc, _ := newRS(t)
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { query = r.URL.Query().Get("t") }))
	t.Cleanup(srv.Close)
	out := svc.Execute(RequestInput{Method: "GET", URL: srv.URL + "/?t={{nowhere}}"})
	if out.Error != "" || query != "{{nowhere}}" {
		t.Errorf("no longer sent as written (%q, %q): update CS-GAP-028 and close B-151", out.Error, query)
	}
}
