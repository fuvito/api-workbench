package serialization

import (
	"strings"
	"testing"
)

func TestOnlyKnownSchemaVersionsAreRead(t *testing.T) {
	parsers := map[string]struct {
		body  string
		parse func([]byte) error
	}{
		"request": {"title: r\nmethod: GET\nurl: http://x\n",
			func(b []byte) error { _, err := ParseRequestFile(b, "r.yaml"); return err }},
		"scenario": {"title: s\nsteps: []\n",
			func(b []byte) error { _, err := ParseScenarioFile(b, "s.yaml"); return err }},
		"environment": {"name: e\nvariables: []\n",
			func(b []byte) error { _, err := ParseEnvironmentFile(b, "e.yaml"); return err }},
		"request written in a step": {"schemaVersion: \"1\"\ntitle: s\nsteps:\n  - label: a\n    request: {schemaVersion: \"9\", method: GET, url: http://x}\n",
			func(b []byte) error { _, err := ParseScenarioFile(b, "s.yaml"); return err }},
	}
	for name, p := range parsers {
		if name == "request written in a step" {
			if err := p.parse([]byte(p.body)); err == nil || !strings.Contains(err.Error(), `schemaVersion "9" is not supported`) {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		for _, bad := range []string{`"9"`, `"2"`, `"1.0"`, `9`} {
			err := p.parse([]byte("schemaVersion: " + bad + "\n" + p.body))
			if err == nil || !strings.Contains(err.Error(), "is not supported") || !strings.Contains(err.Error(), `reads "1"`) {
				t.Errorf("%s, schemaVersion %s: %v", name, bad, err)
			}
		}
		// "1", quoted or not, and no version at all keep working
		for _, good := range []string{"schemaVersion: \"1\"\n", "schemaVersion: 1\n", ""} {
			if err := p.parse([]byte(good + p.body)); err != nil {
				t.Errorf("%s, %q: %v", name, good, err)
			}
		}
	}
	if _, err := ParseScenarioFile([]byte(`{"schemaVersion": "9", "title": "s", "steps": []}`), "s.json"); err == nil {
		t.Error("JSON is refused as YAML is")
	}
}
