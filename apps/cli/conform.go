package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fuvito/api-workbench/packages/core/application"
	"github.com/fuvito/api-workbench/packages/core/checks"
	"github.com/fuvito/api-workbench/packages/core/conformance"
	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/fuvito/api-workbench/packages/core/history"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/reporting"
	"github.com/fuvito/api-workbench/packages/core/serialization"
	"github.com/fuvito/api-workbench/packages/core/telemetry"
	"github.com/spf13/cobra"
)

func conformCmd() *cobra.Command {
	var baseURL string
	var envFile string
	var tagsCSV string
	var pathsCSV string
	var format string
	var outputFile string
	var requestTimeout int
	var maxResponseTime int
	var timeLimits []string
	var failFast bool
	var readOnly bool
	var failOnFlag string
	var setupFile string
	var changedAgainst string
	var explain bool
	var vars []string
	var reports []string
	var showCoverage bool
	var runNegative, listOnly bool
	var baselineFile, writeBaseline string
	var start = time.Now()

	cmd := &cobra.Command{
		Use:   "conform <spec-file-or-url>",
		Short: "Run all spec operations against a live server and validate responses",
		Long: `Parse an OpenAPI 3.0/3.1 spec, execute each declared operation against
a running server, and validate every response against the spec's declared
response schema.

Exit codes:
  0  all operations conformed
  1  one or more violations or execution failures
  2  spec parse or configuration error

With --baseline, violations already recorded in the baseline file are accepted and
only new ones fail the run; --write-baseline records the current violations.

--list sends nothing to the API: it lists every operation with the values its request
needs and the ones nothing provides, and exits 0 once the spec has been read.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listOnly {
				if err := checkListFlags(cmd); err != nil {
					return err
				}
			}
			formatTyped, outputTyped := cmd.Flags().Changed("format"), cmd.Flags().Changed("output")

			args, err := applyProjectConfig(cmd, args)
			if err != nil {
				return err
			}
			if listOnly {
				// What the project file sets for a real run does not apply to a listing.
				if setupFile != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Note: --list sends nothing, so the setup scenario %s is not run; what it would extract is listed as missing.\n", setupFile)
				}
				setupFile, baselineFile, writeBaseline, reports = "", "", "", nil
				if !formatTyped {
					format = "text"
				}
				if !outputTyped {
					outputFile = ""
				}
			}
			if len(args) == 0 {
				return fmt.Errorf("conform needs a spec: pass one, or set spec: in a clrsig.yaml (clrsig init creates one)")
			}
			src := args[0]

			plan, err := planReportsWith([]string{"github"}, format, reports, outputFile)
			if err != nil {
				return err
			}
			pathLimits, err := conformance.ParsePathLimits(timeLimits)
			if err != nil {
				return err
			}
			failOn, err := checks.ParseFailOn(failOnFlag)
			if err != nil {
				return err
			}

			if baselineFile != "" && writeBaseline != "" {
				return fmt.Errorf("use --baseline or --write-baseline, not both")
			}
			if changedAgainst != "" && (strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")) {
				return fmt.Errorf("--changed-against needs a local spec file, not a URL")
			}
			if writeBaseline != "" && (changedAgainst != "" || tagsCSV != "" || pathsCSV != "") {
				return fmt.Errorf("--write-baseline records the whole API, and a run limited by --changed-against, --tags or --paths would drop the other entries: record the baseline with a full run")
			}
			var base *conformance.Baseline
			if baselineFile != "" {
				baseData, err := os.ReadFile(baselineFile)
				if err != nil {
					return fmt.Errorf("reading baseline: %w (record one with --write-baseline %s)", err, baselineFile)
				}
				if base, err = conformance.ParseBaseline(baseData, baselineFile); err != nil {
					return err
				}
			}

			data, specFormat, err := fetchSpec(src)
			if err != nil {
				return err
			}

			spec, err := openapi.ParseBytes(data, specFormat)
			if err != nil {
				return fmt.Errorf("parsing spec: %w", err)
			}

			var env *domain.Environment
			if envFile != "" {
				envData, err := os.ReadFile(envFile)
				if err != nil {
					return fmt.Errorf("reading environment: %w", err)
				}
				env, err = serialization.ParseEnvironmentFile(envData, envFile)
				if err != nil {
					return fmt.Errorf("parsing environment: %w", err)
				}
			}

			if env, err = finishEnvironment(cmd, env, vars); err != nil {
				return err
			}

			var only map[string]bool
			var scope *conformance.Scope
			if changedAgainst != "" {
				if only, scope, err = changedOperationScope(src, changedAgainst, spec); err != nil {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(), scope.Line())
			}

			if listOnly {
				paths := application.SplitList(pathsCSV)
				tags := application.SplitList(tagsCSV)
				list, err := conformance.ListNeeds(spec, baseURL, env, conformance.Options{Tags: tags, Paths: paths, ReadOnly: readOnly, OnlyOperations: only})
				if err != nil {
					return fmt.Errorf("reading the operations of the spec: %w", err)
				}
				data, err := renderNeeds(list, strings.ToLower(format))
				if err != nil {
					return err
				}
				if err := writeReport(cmd, outputFile, data); err != nil {
					return err
				}
				if only == nil {
					if msg := application.NoMatchMessage(tags, paths, list.FilterMatched, len(list.Operations)); msg != "" {
						return fmt.Errorf("%s", msg)
					}
				}
				return nil
			}

			exec := newExecutor()
			if setupFile != "" {
				if env, err = runConformSetup(cmd, exec, setupFile, env, baseURL, vars); err != nil {
					return err
				}
			}
			out := application.RunConformance(context.Background(), application.ConformInput{
				Exec:              exec,
				Spec:              spec,
				BaseURL:           baseURL,
				Env:               env,
				Tags:              application.SplitList(tagsCSV),
				Paths:             application.SplitList(pathsCSV),
				FailFast:          failFast,
				ReadOnly:          readOnly,
				RequestTimeoutMs:  requestTimeout,
				MaxResponseTimeMs: maxResponseTime,
				PathLimits:        pathLimits,
				Coverage:          showCoverage,
				Negative:          runNegative,
				Baseline:          base,
				FailOn:            failOn,
				OnlyOperations:    only,
				Scope:             scope,
			})
			result, cov, negResult := out.Result, out.Coverage, out.Negative

			// Hash the spec bytes to group runs of the same spec in history.
			rawHash := sha256.Sum256(data)
			specHash := hex.EncodeToString(rawHash[:])

			// Record this run to history (silently — a broken db never blocks conformance).
			recordConformRun(src, specHash, result)

			render := func(f string) ([]byte, error) {
				switch f {
				case "github":
					return []byte(reporting.ConformGitHub(result, negResult, githubLocator(src, data))), nil
				case "json":
					b, err := reporting.ConformJSON(result, cov, negResult)
					if err != nil {
						return nil, fmt.Errorf("generating JSON report: %w", err)
					}
					return append(b, '\n'), nil
				case "junit":
					b, err := reporting.ConformJUnit(result, negResult)
					if err != nil {
						return nil, fmt.Errorf("generating JUnit report: %w", err)
					}
					return b, nil
				case "html":
					b, err := reporting.ConformHTML(result, cov, negResult)
					if err != nil {
						return nil, fmt.Errorf("generating HTML report: %w", err)
					}
					return b, nil
				case "markdown":
					return reporting.ConformMarkdown(result, cov, negResult), nil
				}
				text := reporting.ConformTextExplained(result, cov, explain)
				if negResult != nil {
					text += reporting.NegativeText(negResult)
				}
				return []byte(text), nil
			}
			if err := plan.emit(cmd, outputFile, render); err != nil {
				return err
			}

			// Print cumulative coverage in text mode when --coverage is set and
			// there is more than one run recorded for this spec.
			if showCoverage && plan.format == "text" {
				printCumulativeCoverage(cmd, specHash)
			}

			if out.NothingSelected != "" {
				return fmt.Errorf("%s", out.NothingSelected)
			}

			runFailed := out.Failed
			if writeBaseline != "" {
				if err := conformance.WriteBaselineFile(writeBaseline, conformance.NewBaseline(out.Violations)); err != nil {
					return fmt.Errorf("writing baseline: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Baseline written: %d violation%s recorded in %s\n", len(out.Violations), plural(len(out.Violations)), writeBaseline)
				runFailed = false // recording the current state accepts it
			}
			if out.Diff != nil {
				fmt.Fprint(cmd.ErrOrStderr(), out.Diff.Text())
			}
			tel := newTelemetryClient()
			telResult := "pass"
			if runFailed {
				telResult = "fail"
			}
			passed, failedCount, _ := result.Counts()
			tel.Record(telemetry.Event{
				Command:        "conform",
				Result:         telResult,
				DurationMs:     time.Since(start).Milliseconds(),
				AssertionCount: passed + failedCount,
			})
			if runFailed {
				return &exitError{1}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&baseURL, "base-url", "b", "", "Base URL to send requests to (overrides spec servers[0])")
	cmd.Flags().StringVarP(&envFile, "env", "e", "", "Path to environment file for variable resolution")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "Set or override an environment variable for this run: name=value (repeatable)")
	cmd.Flags().StringArrayVar(&reports, "report", nil, "Also write a report to a file: format=file, for example junit=reports/conform.xml (repeatable)")
	cmd.Flags().StringVar(&tagsCSV, "tags", "", "Comma-separated list of OpenAPI tags to include")
	cmd.Flags().StringVar(&pathsCSV, "paths", "", "Comma-separated path prefixes to include; each matches the start of the path as the spec writes it (/latency/ also selects /latency/{ms})")
	cmd.Flags().StringVarP(&format, "format", "f", "text", formatUsage+", or github (workflow commands that annotate a pull request, with the check ID in the title and the line of the operation in the spec)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write report to file (parent directories created automatically)")
	cmd.Flags().IntVar(&requestTimeout, "request-timeout", application.DefaultConformRequestTimeoutMs, "Per-request timeout in milliseconds")
	cmd.Flags().IntVar(&maxResponseTime, "max-response-time", 0, "Fail an operation whose response takes longer than this many milliseconds (the response is still compared with the spec; 0 = no limit)")
	cmd.Flags().StringArrayVar(&timeLimits, "max-response-time-for", nil, "A different response-time limit: /prefix=ms, tag:name=ms or \"GET /path=ms\" (repeatable; an operation rule wins over a path prefix, the longest prefix over tags, and the strictest tag holds; 0 turns the limit off)")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop after the first violation or execution error")
	cmd.Flags().BoolVar(&explain, "explain", false, "Explain each failure in plain language (what the spec expected, what the server sent, why it fails, how to fix it); JSON, HTML, Markdown and JUnit always include it")
	cmd.Flags().StringVar(&changedAgainst, "changed-against", "", "Check only the operations that are new or changed compared with the spec at this git ref (for pull requests; needs git and a local spec file; a full run on main stays the safety net)")
	cmd.Flags().StringVar(&setupFile, "setup", "", "A scenario to run before the check, usually a login: what its steps extract is available to the generated requests as {{name}} (never printed; credential-like names are masked); if it fails the check does not start")
	cmd.Flags().StringVar(&failOnFlag, "fail-on", "failure", "Fail the run on findings of this level or worse: failure (the default) or warning; warnings are reported but do not fail the run unless this is warning")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Send only read requests (GET, HEAD); every other operation is skipped and never sent")
	cmd.Flags().BoolVar(&showCoverage, "coverage", false, "Append a coverage summary showing which operations received a response")
	cmd.Flags().StringVar(&baselineFile, "baseline", "", "Accept the violations recorded in this baseline file; only new violations fail the run")
	cmd.Flags().StringVar(&writeBaseline, "write-baseline", "", "Record the current violations in this file (YAML or JSON by extension) and exit 0")
	cmd.Flags().BoolVar(&listOnly, "list", false, "List the operations and the values each needs, without sending anything to the API (text or json)")
	cmd.Flags().BoolVar(&runNegative, "negative", false, "Also run negative probes: drop required fields, wrong types, missing auth, missing params")
	return cmd
}

// recordConformRun persists per-operation coverage to history.db.
// Errors are silently discarded so a broken database never blocks conformance output.
func recordConformRun(specPath, specHash string, result *conformance.ConformResult) {
	dbPath, err := history.DefaultPath()
	if err != nil {
		return
	}
	store, err := history.Open(dbPath)
	if err != nil {
		return
	}
	defer store.Close()

	ops := make([]history.ConformOp, len(result.Operations))
	for i, op := range result.Operations {
		ops[i] = history.ConformOp{
			OperationID: op.OperationID,
			Method:      op.Method,
			Path:        op.Path,
			Covered:     op.StatusCode > 0,
		}
	}
	_ = store.RecordConformRun(specPath, specHash, ops)
}

// printCumulativeCoverage writes the cross-run coverage summary to cmd's stdout
// when more than one run has been recorded for the given spec hash.
func printCumulativeCoverage(cmd *cobra.Command, specHash string) {
	dbPath, err := history.DefaultPath()
	if err != nil {
		return
	}
	store, err := history.Open(dbPath)
	if err != nil {
		return
	}
	defer store.Close()

	rep, err := store.CumulativeCoverage(specHash)
	if err != nil || rep == nil || rep.RunCount < 2 {
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nCumulative coverage (%d runs):  %d/%d (%.1f%%)\n",
		rep.RunCount, len(rep.EverCoveredOps), rep.TotalOperations, rep.Percent)
	if len(rep.NeverCoveredOps) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Never covered: %s\n", conformance.SpecStyle(strings.Join(rep.NeverCoveredOps, ", ")))
	}
}

// fetchSpec retrieves spec bytes and detected format from a file path or URL.
func fetchSpec(src string) (data []byte, format string, err error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, fetchErr := client.Get(src)
		if fetchErr != nil {
			return nil, "", fmt.Errorf("fetching spec: %w", fetchErr)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("server returned %d", resp.StatusCode)
		}
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("reading response: %w", err)
		}
		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "json") || strings.HasSuffix(strings.ToLower(src), ".json") {
			format = "json"
		} else {
			format = "yaml"
		}
		return data, format, nil
	}
	data, err = os.ReadFile(src)
	if err != nil {
		return nil, "", fmt.Errorf("reading spec: %w", err)
	}
	format = strings.TrimPrefix(strings.ToLower(filepath.Ext(src)), ".")
	if format == "yml" {
		format = "yaml"
	}
	return data, format, nil
}
