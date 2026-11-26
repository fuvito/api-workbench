package main

import (
	"runtime"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/clibuild"
)

// unsavedBundleName is the file the copied command names when the bundle was pasted.
const unsavedBundleName = "bundle.yaml"

// BundleCommandIO is the clrsig command that does what the import dialog would.
type BundleCommandIO struct {
	Command string `json:"command"`
	Shell   string `json:"shell"`
	// Note says what the person must do before the command works, or what the shell may change.
	Note  string `json:"note,omitempty"`
	Error string `json:"error,omitempty"`
}

// BundleImportCommand returns the command for the dialog's choices. filePath is the file the text
// came from, or "" when it was pasted or edited since.
func (s *BundleService) BundleImportCommand(filePath, specFile, strategy string) BundleCommandIO {
	ws := s.workspace.workspacePath
	if ws == "" {
		return BundleCommandIO{Error: "no workspace open — open or create a workspace first"}
	}
	shell := clibuild.Bash
	if runtime.GOOS == "windows" {
		shell = clibuild.PowerShell
	}
	var notes []string
	file := filePath
	if file == "" {
		file = unsavedBundleName
		notes = append(notes, "Save the bundle's text as "+unsavedBundleName+" in the folder you run this from.")
	}
	argv := []string{clibuild.Program, "bundle", "import", file, "--workspace", ws}
	if spec := strings.TrimSpace(specFile); spec != "" {
		argv = append(argv, "--spec-file", spec)
	}
	if strategy != "" && strategy != "rename" {
		argv = append(argv, "--on-conflict", strategy)
	}
	line, warnings := clibuild.CommandLine(shell, argv)
	return BundleCommandIO{Command: line, Shell: string(shell), Note: strings.Join(append(notes, warnings...), " ")}
}
