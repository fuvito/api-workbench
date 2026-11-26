package checks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fuvito/api-workbench/packages/core/checks"
)

func TestParseCatalogReadsEveryRowOfTheDocumentWhole(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "apps", "desktop", "wails", "help", "what-we-check.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(string(data), "\r\n", "\n")
	rows := checks.ParseCatalog(doc)
	if want := len(reDocRow.FindAllString(doc, -1)); len(rows) != want {
		t.Fatalf("%d rows read, the document has %d", len(rows), want)
	}
	// the number of columns of each row's line must be its table's, or a pipe was not escaped
	columns := map[string]int{}
	var header int
	for _, line := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(line, "|") {
			header = 0
			continue
		}
		n := strings.Count(line, "|") - strings.Count(line, `\|`)
		if header == 0 {
			header = n
		} else if m := reDocRow.FindStringSubmatch(line); m != nil {
			columns[m[1]] = n
			if n != header {
				t.Errorf("%s has %d columns and its table %d", m[1], n-1, header-1)
			}
		}
	}
	bySeverity := map[string]checks.Severity{}
	for _, r := range rows {
		bySeverity[r.ID] = r.Severity
		if r.Title == "" || r.Section == "" || len(r.Cells) == 0 {
			t.Errorf("%s is incomplete: %+v", r.ID, r)
		}
	}
	for _, c := range checks.All() {
		if bySeverity[c.ID] != c.Severity {
			t.Errorf("%s: severity %q, the check data says %q", c.ID, bySeverity[c.ID], c.Severity)
		}
	}
	if got, ok := bySeverity["CS-GAP-001"]; !ok || got != "" {
		t.Errorf("a known gap has no severity: %q %v", got, ok)
	}
}

func TestParseCatalogCells(t *testing.T) {
	rows := checks.ParseCatalog("## One\r\n\r\n| ID | Title | State | Note |\r\n|---|---|---|---|\r\n| CS-RUN-001 | T | a \\| b |  |\r\n| other | x | y | z |\r\n\r\ntext\r\n| A | B |\r\n|---|---|\r\n| 1 | 2 |\r\n")
	if len(rows) != 1 {
		t.Fatalf("%+v", rows)
	}
	r := rows[0]
	if r.ID != "CS-RUN-001" || r.Title != "T" || r.Section != "One" || r.Severity != checks.Failure {
		t.Errorf("%+v", r)
	}
	if len(r.Cells) != 1 || r.Cells[0] != (checks.Cell{Label: "State", Text: "a | b"}) {
		t.Errorf("cells: %+v", r.Cells)
	}
}
