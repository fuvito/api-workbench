package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/fuvito/api-workbench/packages/core/scenario"
	"github.com/fuvito/api-workbench/packages/core/serialization"
	"github.com/fuvito/api-workbench/packages/core/variables"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	var format string
	var outputFile string
	var noRecursive bool
	var strict bool
	var envFile string
	var specFile string

	cmd := &cobra.Command{
		Use:   "validate <scenario-file-or-folder> [more...]",
		Short: "Check scenario files for errors without executing them",
		Long: `Check scenario files for mistakes without sending anything. Exit code 0: every file is valid; 1: a file has
errors; 2: a file could not be read.

With --strict, also report scenarios that are valid but weak, as warnings that fail the check:
  - a step that has no assertions and extracts nothing (it passes whenever the request is answered)
  - a value extracted by a step that no later step uses
  - with --env, a {{name}} that nothing defines (not a scenario variable, not extracted earlier, not in the environment)

With --spec <openapi-file>, also check the spec itself, before conformance results are read against it: an operation
without an operationId, a schema whose required lists a property it does not declare, an example that breaks its
format. Errors fail the check; warnings are shown as notes and fail it only with --strict. The scenarios are optional
when --spec is given.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			args, err := applyProjectConfig(cmd, args)
			if err != nil {
				return err
			}
			if len(args) == 0 && specFile == "" {
				return fmt.Errorf("validate needs a scenario file or folder (or --spec <openapi-file>): pass one, or set run.scenarios in a clrsig.yaml")
			}
			type result struct {
				path     string
				notes    []scenario.Issue
				issues   []scenario.Issue
				warnings []scenario.Issue
				err      error
				// unreadable: the file itself could not be read, which exits 2 rather than 1
				unreadable bool
			}
			var lintEnv *domain.Environment
			envProblems := false
			if strict && envFile != "" {
				envData, err := os.ReadFile(envFile)
				if err != nil {
					return fmt.Errorf("reading environment: %w", err)
				}
				if lintEnv, err = serialization.ParseEnvironmentFile(envData, envFile); err != nil {
					return fmt.Errorf("parsing environment: %w", err)
				}
				for _, ref := range variables.MissingEnv(lintEnv) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", ref.Message())
					envProblems = true
				}
			}

			var files []string
			if len(args) > 0 {
				var err error
				if files, err = expandScenarioArgs(cmd, args, noRecursive); err != nil {
					return err
				}
			}

			results := make([]result, 0, len(files)+1)
			if specFile != "" {
				errs, warns, serr := lintSpecFile(specFile)
				r := result{path: specFile, err: serr}
				if serr == nil {
					r.issues = errs
					if strict {
						r.warnings = warns
					} else {
						r.notes = warns
					}
				}
				results = append(results, r)
			}
			for _, path := range files {
				data, err := os.ReadFile(path)
				if err != nil {
					results = append(results, result{path: path, err: fmt.Errorf("reading file: %w", err), unreadable: true})
					continue
				}
				sc, err := serialization.ParseScenarioFile(data, path)
				if err != nil {
					results = append(results, result{path: path, err: fmt.Errorf("parse error: %w", err)})
					continue
				}
				r := result{path: path, issues: scenario.Validate(sc, filepath.Dir(path))}
				if strict {
					dir := filepath.Dir(path)
					scenario.ResolveRequestRefs(sc, dir, filepath.Join(dir, "..", "requests"))
					r.warnings = scenario.Lint(sc, scenario.LintOptions{Env: lintEnv})
					if sc.SpecFile != "" && !strings.Contains(sc.SpecFile, "://") {
						spec := sc.SpecFile
						if !filepath.IsAbs(spec) {
							spec = filepath.Join(dir, spec)
						}
						if _, err := os.Stat(spec); err != nil {
							r.warnings = append(r.warnings, scenario.Issue{Message: fmt.Sprintf("specFile %q does not exist (looked for %s): its responses cannot be checked against a spec", sc.SpecFile, filepath.ToSlash(spec))})
						}
					}
				}
				results = append(results, r)
			}

			allValid := !envProblems
			for _, r := range results {
				if r.err != nil || len(r.issues) > 0 || len(r.warnings) > 0 {
					allValid = false
					break
				}
			}

			var reportBytes []byte
			switch strings.ToLower(format) {
			case "json":
				type jsonIssue struct {
					Step    int    `json:"step"`
					Name    string `json:"stepName,omitempty"`
					Message string `json:"message"`
				}
				type jsonResult struct {
					Path   string      `json:"path"`
					Valid  bool        `json:"valid"`
					Error  string      `json:"error,omitempty"`
					Issues []jsonIssue `json:"issues"`
					// Warnings are the findings of --strict; with --strict they make the file not valid.
					Warnings []jsonIssue `json:"warnings,omitempty"`
					// Notes are spec findings that do not fail the check without --strict.
					Notes []jsonIssue `json:"notes,omitempty"`
				}
				out := make([]jsonResult, 0, len(results))
				for _, r := range results {
					jr := jsonResult{Path: r.path, Valid: r.err == nil && len(r.issues) == 0 && len(r.warnings) == 0, Issues: []jsonIssue{}}
					if r.err != nil {
						jr.Error = r.err.Error()
					}
					for _, iss := range r.issues {
						jr.Issues = append(jr.Issues, jsonIssue{Step: iss.Step, Name: iss.Name, Message: iss.Message})
					}
					for _, w := range r.warnings {
						jr.Warnings = append(jr.Warnings, jsonIssue{Step: w.Step, Name: w.Name, Message: w.Message})
					}
					for _, n := range r.notes {
						jr.Notes = append(jr.Notes, jsonIssue{Step: n.Step, Name: n.Name, Message: n.Message})
					}
					out = append(out, jr)
				}
				b, _ := json.MarshalIndent(out, "", "  ")
				reportBytes = append(b, '\n')

			default:
				var sb strings.Builder
				for _, r := range results {
					if r.err != nil {
						fmt.Fprintf(&sb, "%s\n  PARSE ERROR: %s\n\n", r.path, r.err)
						continue
					}
					if len(r.issues) == 0 && len(r.warnings) == 0 {
						fmt.Fprintf(&sb, "%s  OK\n", r.path)
						for _, n := range r.notes {
							fmt.Fprintf(&sb, "  note: %s\n", n)
						}
					} else {
						fmt.Fprintf(&sb, "%s\n", r.path)
						for _, iss := range r.issues {
							fmt.Fprintf(&sb, "  %s\n", iss)
						}
						for _, w := range r.warnings {
							fmt.Fprintf(&sb, "  warning: %s\n", w)
						}
						for _, n := range r.notes {
							fmt.Fprintf(&sb, "  note: %s\n", n)
						}
						sb.WriteByte('\n')
					}
				}
				if len(results) > 1 {
					valid := 0
					for _, r := range results {
						if r.err == nil && len(r.issues) == 0 && len(r.warnings) == 0 {
							valid++
						}
					}
					fmt.Fprintf(&sb, "%d file(s) checked  %d valid  %d invalid\n",
						len(results), valid, len(results)-valid)
				}
				reportBytes = []byte(sb.String())
			}

			if outputFile != "" {
				if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
					return fmt.Errorf("creating output directory: %w", err)
				}
				if err := os.WriteFile(outputFile, reportBytes, 0o644); err != nil {
					return fmt.Errorf("writing output file: %w", err)
				}
			} else {
				fmt.Fprint(cmd.OutOrStdout(), string(reportBytes))
			}

			for _, r := range results {
				if r.unreadable {
					return &exitError{2}
				}
			}
			if !allValid {
				return &exitError{1}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&noRecursive, "no-recursive", false, "Only look at the top level of each folder, not its sub-folders")
	cmd.Flags().StringVar(&specFile, "spec", "", "Also check this OpenAPI spec itself (missing operationId, required properties that are not declared, examples that break their format); errors fail the check, warnings only with --strict")
	cmd.Flags().BoolVar(&strict, "strict", false, "Also report valid but weak scenarios (steps that check nothing, extractions nobody uses, with --env names nothing defines) as warnings that fail the check")
	cmd.Flags().StringVarP(&envFile, "env", "e", "", "With --strict: the environment the scenarios will run against, so names nothing defines can be found")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write report to file (parent directories created automatically)")
	return cmd
}
