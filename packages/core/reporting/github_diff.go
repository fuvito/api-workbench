package reporting

import (
	"fmt"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/redact"
)

// DiffGitHub renders a spec comparison as GitHub Actions workflow commands: an error for each change
// that fails at failOn, a warning for another potentially breaking one and a notice for the rest,
// then one notice with the counts. loc points at the operation in the new spec; a removed operation
// has no line there, so its annotation is on the file. GitHub shows only the first annotations of
// each level for a step, so the counts and the log hold the whole result.
func DiffGitHub(result *openapi.DiffResult, changes []openapi.DiffChange, failOn openapi.FailOn, loc Locator) string {
	var b strings.Builder
	annotate := func(level, file string, line int, title, message string) {
		props := ""
		if file != "" {
			props += "file=" + escapeProperty(file)
			if line > 0 {
				props += fmt.Sprintf(",line=%d", line)
			}
			props += ","
		}
		props += "title=" + escapeProperty(title)
		fmt.Fprintf(&b, "::%s %s::%s\n", level, props, escapeData(message))
	}
	for _, c := range changes {
		level := "notice"
		switch {
		case c.Fails(failOn):
			level = "error"
		case c.Level == "WARN":
			level = "warning"
		}
		file, line := "", 0
		if loc != nil && c.Path != "" {
			file, line = loc(c.Method, c.Path)
		}
		title := strings.TrimSpace(c.ID + " " + c.Method + " " + c.Path)
		annotate(level, file, line, title, redact.Credentials(c.Text))
	}
	counts := map[string]int{}
	for _, c := range changes {
		counts[c.Level]++
	}
	annotate("notice", "", 0, "Clear Signal diff", fmt.Sprintf("%d change(s): %d breaking, %d potentially breaking, %d informational. %d at or above --fail-on %s.",
		len(changes), counts["ERR"], counts["WARN"], counts["INFO"], result.FailCount(failOn), failOn))
	return b.String()
}
