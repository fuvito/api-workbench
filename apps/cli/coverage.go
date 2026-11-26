package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fuvito/api-workbench/packages/core/coverage"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/reporting"
	"github.com/spf13/cobra"
)

func coverageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "View and manage accumulated spec coverage for a workspace",
	}
	cmd.AddCommand(coverageListCmd())
	cmd.AddCommand(coverageQualityCmd())
	cmd.AddCommand(coverageResetCmd())
	return cmd
}

// requireWorkspace refuses a folder that is not there: reading it would look like "nothing recorded",
// which hides a misspelled name.
func requireWorkspace(path string) error {
	if path == "" {
		return fmt.Errorf("--workspace is required")
	}
	if !isDir(path) {
		return fmt.Errorf("workspace %q is not a folder: check the name given to --workspace", path)
	}
	return nil
}

// qualityGate is the exit status of coverage quality. With --fail-on-gaps, nothing recorded fails
// too: a gate that has looked at nothing has not passed.
func qualityGate(cmd *cobra.Command, failOnGaps bool, runs int, hasGaps bool) error {
	if !failOnGaps {
		return nil
	}
	if runs == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "failed: --fail-on-gaps cannot pass when no coverage was recorded")
		return &exitError{1}
	}
	if hasGaps {
		return &exitError{1}
	}
	return nil
}

func coverageListCmd() *cobra.Command {
	var workspacePath string
	var format string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accumulated coverage for all specs in a workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := workspacePath
			if err := requireWorkspace(ws); err != nil {
				return err
			}
			reports := coverage.ListAll(ws)
			if f := strings.ToLower(format); isContractsFormat(f) {
				data, err := contractsListReport(f, reports)
				if err != nil {
					return err
				}
				return writeReport(cmd, outputFile, data)
			}
			var out bytes.Buffer
			switch strings.ToLower(format) {
			case "json":
				type jsonEntry struct {
					SpecFile   string   `json:"specFile"`
					Covered    int      `json:"covered"`
					Total      int      `json:"total"`
					Percent    float64  `json:"percent"`
					TotalRuns  int      `json:"totalRuns"`
					PassedRuns int      `json:"passedRuns"`
					FailedRuns int      `json:"failedRuns"`
					LastRunAt  string   `json:"lastRunAt,omitempty"`
					Exercised  []string `json:"exercised"`
					// the four totals of the Contracts page: Covered + NotCovered = Total, Passing + Failing = Covered
					NotCovered int `json:"notCovered"`
					Passing    int `json:"passing"`
					Failing    int `json:"failing"`
					NoResult   int `json:"noResult"`
				}
				entries := make([]jsonEntry, len(reports))
				for i, r := range reports {
					lastRun := ""
					if !r.LastRunAt.IsZero() {
						lastRun = r.LastRunAt.Format(time.RFC3339)
					}
					entries[i] = jsonEntry{
						SpecFile:   r.SpecFile,
						Covered:    r.Covered,
						Total:      r.Total,
						Percent:    r.Percent,
						TotalRuns:  r.TotalRuns,
						PassedRuns: r.PassedRuns,
						FailedRuns: r.FailedRuns,
						LastRunAt:  lastRun,
						Exercised:  r.Exercised,
						NotCovered: r.Outcomes().NotCovered,
						Passing:    r.Outcomes().Passing,
						Failing:    r.Outcomes().Failing,
						NoResult:   r.Outcomes().NoResult,
					}
				}
				b, _ := json.MarshalIndent(entries, "", "  ")
				fmt.Fprintln(&out, string(b))
			default:
				if len(reports) == 0 {
					fmt.Fprintln(&out, "No coverage data found in workspace.")
				}
				for _, r := range reports {
					lastRun := ""
					if !r.LastRunAt.IsZero() {
						lastRun = "  last run " + r.LastRunAt.Format("2006-01-02 15:04")
					}
					fmt.Fprintf(&out, "%s\n  %s\n  scenario runs: %d run(s) · %d passed · %d failed%s\n\n",
						r.SpecFile, reporting.OutcomesLine(r.Outcomes()),
						r.TotalRuns, r.PassedRuns, r.FailedRuns, lastRun)
				}
			}
			return writeReport(cmd, outputFile, out.Bytes())
		},
	}
	cmd.Flags().StringVarP(&workspacePath, "workspace", "w", "", "Workspace directory (required)")
	cmd.Flags().StringVarP(&format, "format", "f", "text", contractsFormatUsage)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write the report to a file (parent directories created automatically)")
	return cmd
}

func coverageQualityCmd() *cobra.Command {
	var workspacePath string
	var format string
	var outputFile string
	var failOnGaps bool

	cmd := &cobra.Command{
		Use:   "quality <spec-file-or-url>",
		Short: "Show coverage quality indicators: gaps, untested error paths, and auth coverage",
		Long: `Analyse accumulated workspace coverage against the spec to answer
"what have you NOT proven?" rather than just counting executed operations.

Reports four quality signals:
  NEVER EXERCISED        — operations never reached by any recorded run
  HAPPY PATH ONLY        — exercised but only 2xx responses ever observed
  SECURED NEVER AUTHED   — auth-required ops never tested with credentials
  UNTESTED ERROR PATHS   — declared 4xx/5xx responses never triggered

The gaps are signals, not failures: the command exits 0 whether or not it found any. Add
--fail-on-gaps to make a gap fail the step (for example to keep a coverage bar in CI). With it,
the step also fails when nothing was recorded for the spec.

Exit codes:
  0  the report was produced (and, with --fail-on-gaps, no gap was found)
  1  --fail-on-gaps was given and one or more gaps were detected, or nothing was recorded
  2  spec parse or configuration error, or the workspace folder does not exist`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireWorkspace(workspacePath); err != nil {
				return err
			}

			specArg := args[0]
			data, specFmt, err := fetchSpec(specArg)
			if err != nil {
				return err
			}
			spec, err := openapi.ParseBytes(data, specFmt)
			if err != nil {
				return fmt.Errorf("parsing spec: %w", err)
			}

			title, version := spec.Info()
			meta := openapi.ExtractSpecMeta(spec)

			allOps := spec.AllOperations()
			allOpStrs := make([]string, len(allOps))
			for i, op := range allOps {
				allOpStrs[i] = op.String()
			}

			stored := coverage.Load(workspacePath, specArg, ".")
			report := coverage.BuildReport(stored, allOpStrs)

			if f := strings.ToLower(format); isContractsFormat(f) {
				q := coverage.ComputeQuality(report, meta.DeclaredStatusCodes, meta.SecuredOps)
				data, err := contractsReport(f, specArg, report, q)
				if err != nil {
					return err
				}
				if err := writeReport(cmd, outputFile, data); err != nil {
					return err
				}
				return qualityGate(cmd, failOnGaps, report.TotalRuns, qualityHasGaps(q))
			}

			// A person gets a sentence when nothing was recorded; a program asking for JSON gets JSON.
			if report.TotalRuns == 0 && strings.ToLower(format) != "json" {
				msg := fmt.Sprintf("No coverage data found for %s in %s.\nRun scenarios with --workspace and --spec to accumulate coverage.\n", specArg, workspacePath)
				if err := writeReport(cmd, outputFile, []byte(msg)); err != nil {
					return err
				}
				return qualityGate(cmd, failOnGaps, 0, false)
			}
			var out bytes.Buffer

			q := coverage.ComputeQuality(report, meta.DeclaredStatusCodes, meta.SecuredOps)
			hasGaps := len(q.NeverExercised) > 0 || len(q.HappyPathOnly) > 0 ||
				len(q.SecuredNeverAuthed) > 0 || len(q.ErrorsNeverTriggered) > 0

			switch strings.ToLower(format) {
			case "json":
				type jsonGap struct {
					Operation string `json:"operation"`
					Missing   []int  `json:"missing"`
				}
				type jsonTotals struct {
					Total      int `json:"total"`
					Covered    int `json:"covered"`
					NotCovered int `json:"notCovered"`
					Passing    int `json:"passing"`
					Failing    int `json:"failing"`
					NoResult   int `json:"noResult"`
				}
				o := report.Outcomes()
				type jsonOut struct {
					SpecTitle            string     `json:"specTitle"`
					SpecVersion          string     `json:"specVersion"`
					Workspace            string     `json:"workspace"`
					TotalRuns            int        `json:"totalRuns"`
					Totals               jsonTotals `json:"totals"`
					HasGaps              bool       `json:"hasGaps"`
					NeverExercised       []string   `json:"neverExercised"`
					HappyPathOnly        []string   `json:"happyPathOnly"`
					SecuredNeverAuthed   []string   `json:"securedNeverAuthed"`
					ErrorsNeverTriggered []jsonGap  `json:"errorsNeverTriggered"`
				}
				var gaps []jsonGap
				for _, g := range q.ErrorsNeverTriggered {
					gaps = append(gaps, jsonGap{Operation: g.Operation, Missing: g.Missing})
				}
				doc := jsonOut{
					SpecTitle:            title,
					SpecVersion:          version,
					Workspace:            workspacePath,
					TotalRuns:            report.TotalRuns,
					Totals:               jsonTotals{o.Total, o.Covered, o.NotCovered, o.Passing, o.Failing, o.NoResult},
					HasGaps:              hasGaps,
					NeverExercised:       q.NeverExercised,
					HappyPathOnly:        q.HappyPathOnly,
					SecuredNeverAuthed:   q.SecuredNeverAuthed,
					ErrorsNeverTriggered: gaps,
				}
				b, _ := json.MarshalIndent(doc, "", "  ")
				fmt.Fprintln(&out, string(b))

			default:
				fmt.Fprintf(&out, "Coverage quality — %s  %s\n", title, version)
				fmt.Fprintf(&out, "Workspace: %s  (%d run(s))\n%s\n\n",
					workspacePath, report.TotalRuns, reporting.OutcomesLine(report.Outcomes()))

				if !hasGaps {
					fmt.Fprintln(&out, "No gaps found — all operations exercised with error paths triggered and auth tested.")
					break
				}

				if len(q.NeverExercised) > 0 {
					fmt.Fprintf(&out, "NEVER EXERCISED (%d):\n", len(q.NeverExercised))
					for _, op := range q.NeverExercised {
						fmt.Fprintf(&out, "  %s\n", op)
					}
					fmt.Fprintln(&out)
				}

				if len(q.HappyPathOnly) > 0 {
					fmt.Fprintf(&out, "HAPPY PATH ONLY — no error responses ever triggered (%d):\n", len(q.HappyPathOnly))
					for _, op := range q.HappyPathOnly {
						var missing []string
						for _, gap := range q.ErrorsNeverTriggered {
							if gap.Operation == op {
								for _, code := range gap.Missing {
									missing = append(missing, fmt.Sprintf("%d", code))
								}
								break
							}
						}
						if len(missing) > 0 {
							fmt.Fprintf(&out, "  %-35s  declared errors: %s\n", op, strings.Join(missing, ", "))
						} else {
							fmt.Fprintf(&out, "  %s\n", op)
						}
					}
					fmt.Fprintln(&out)
				}

				if len(q.SecuredNeverAuthed) > 0 {
					fmt.Fprintf(&out, "SECURED — never tested with credentials (%d):\n", len(q.SecuredNeverAuthed))
					for _, op := range q.SecuredNeverAuthed {
						fmt.Fprintf(&out, "  %s\n", op)
					}
					fmt.Fprintln(&out)
				}

				if len(q.ErrorsNeverTriggered) > 0 {
					fmt.Fprintf(&out, "UNTESTED ERROR PATHS (%d operations):\n", len(q.ErrorsNeverTriggered))
					for _, gap := range q.ErrorsNeverTriggered {
						var codes []string
						for _, c := range gap.Missing {
							codes = append(codes, fmt.Sprintf("%d", c))
						}
						fmt.Fprintf(&out, "  %-35s  → %s never triggered\n", gap.Operation, strings.Join(codes, ", "))
					}
					fmt.Fprintln(&out)
				}

				total := len(q.NeverExercised) + len(q.HappyPathOnly) + len(q.SecuredNeverAuthed) + len(q.ErrorsNeverTriggered)
				fmt.Fprintf(&out, "%d gap(s) found — add scenarios that trigger error paths and test authenticated+unauthenticated flows\n", total)
			}

			if err := writeReport(cmd, outputFile, out.Bytes()); err != nil {
				return err
			}
			return qualityGate(cmd, failOnGaps, report.TotalRuns, hasGaps)
		},
	}
	cmd.Flags().StringVarP(&workspacePath, "workspace", "w", "", "Workspace directory (required)")
	cmd.Flags().StringVarP(&format, "format", "f", "text", contractsFormatUsage)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write the report to a file (parent directories created automatically)")
	cmd.Flags().BoolVar(&failOnGaps, "fail-on-gaps", false, "Exit 1 when the report finds a gap (by default the gaps are only reported)")
	return cmd
}

func coverageResetCmd() *cobra.Command {
	var workspacePath string

	cmd := &cobra.Command{
		Use:   "reset <spec-file-or-url>",
		Short: "Delete the accumulated coverage data for a specific spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := workspacePath
			if err := requireWorkspace(ws); err != nil {
				return err
			}
			specRef := args[0]
			if !coverage.Has(ws, specRef, ".") {
				fmt.Fprintf(cmd.OutOrStdout(), "No coverage data for %s in %s: nothing was reset.\n", specRef, ws)
				return nil
			}
			if err := coverage.Reset(ws, specRef, "."); err != nil {
				return fmt.Errorf("resetting coverage: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Coverage reset for %s\n", specRef)
			return nil
		},
	}
	cmd.Flags().StringVarP(&workspacePath, "workspace", "w", "", "Workspace directory (required)")
	return cmd
}
