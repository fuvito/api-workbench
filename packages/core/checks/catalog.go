package checks

import (
	"regexp"
	"strings"
)

// Row is one row of the catalog of what is checked, for showing it as a table.
type Row struct {
	ID    string
	Title string
	// Section is the heading the row stands under.
	Section string
	// Severity is that of the check with this ID; empty for a row that is not a finding of a run,
	// such as a known gap.
	Severity Severity
	// Cells are the row's other columns, in order. They are Markdown.
	Cells []Cell
}

// Cell is one column of a row.
type Cell struct {
	Label string
	Text  string
}

var rowID = regexp.MustCompile(`^CS-[A-Z]+-\d{3}$`)

// ParseCatalog reads the rows of the catalog document (what-we-check.md): the rows of every table
// whose first column is the ID and second the title. The document is where each row is written;
// the severity comes from the check data.
func ParseCatalog(markdown string) []Row {
	severity := map[string]Severity{}
	for _, c := range All() {
		severity[c.ID] = c.Severity
	}
	var rows []Row
	var section string
	var header []string
	for _, line := range strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			section, header = strings.TrimSpace(strings.TrimLeft(line, "#")), nil
			continue
		}
		if !strings.HasPrefix(line, "|") {
			header = nil
			continue
		}
		cells := tableCells(line)
		if header == nil {
			header = cells
			continue
		}
		if len(cells) < 2 || !rowID.MatchString(cells[0]) {
			continue
		}
		row := Row{ID: cells[0], Title: cells[1], Section: section, Severity: severity[cells[0]]}
		for i := 2; i < len(cells) && i < len(header); i++ {
			if cells[i] != "" {
				row.Cells = append(row.Cells, Cell{Label: header[i], Text: cells[i]})
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// tableCells splits a Markdown table line. A pipe written \| is part of its cell.
func tableCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	var cells []string
	var cell strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			cell.WriteByte('|')
			i++
		case line[i] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteByte(line[i])
		}
	}
	return append(cells, strings.TrimSpace(cell.String()))
}
