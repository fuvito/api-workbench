package main

import (
	"strings"
	"testing"

	"github.com/fuvito/api-workbench/packages/core/importer"
)

func TestDesktopImport_PassesWarningsToTheApp(t *testing.T) {
	isolateHome(t)
	name, requests, err := importer.ParsePostmanCollection([]byte(`{"info": {"name": "c"}, "item": [{"name": "a",
		"request": {"method": "GET", "url": "http://x"},
		"event": [{"listen": "test", "script": {"exec": ["pm.test('ok')"]}}]},
		{"name": "plain", "request": {"method": "GET", "url": "http://x"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	result := NewImportService(newWS(t, t.TempDir())).saveImportedRequests(name, requests)
	if len(result.Imported) != 2 || len(result.Warnings) != 1 || !strings.HasPrefix(result.Warnings[0], "a: a Postman test script") {
		t.Errorf("%+v", result)
	}
}
