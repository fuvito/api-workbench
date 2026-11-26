package clibuild

import "strings"

// CommandLine writes argv as one line for shell, quoting each argument as the CLI Command Builder
// does. warnings names what the shell may still change.
func CommandLine(shell Shell, argv []string) (line string, warnings []string) {
	parts := make([]string, len(argv))
	for i, a := range argv {
		text, warn := quote(shell, a)
		parts[i] = text
		if warn != "" {
			warnings = append(warnings, a+" "+warn)
		}
	}
	return strings.Join(parts, " "), warnings
}
