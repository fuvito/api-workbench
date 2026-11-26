package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/importer"
	"github.com/fuvito/api-workbench/packages/core/openapi"
	"github.com/fuvito/api-workbench/packages/core/serialization"
	"github.com/spf13/cobra"
)

func generateCmd() *cobra.Command {
	var baseURL string
	var outputDir string
	var format string
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "generate <spec-file-or-url>",
		Short: "Generate request files from an OpenAPI specification",
		Long: `Parse an OpenAPI 3.0/3.1 spec and write one YAML request file per operation.

The argument may be a local file path or an http/https URL.

Each generated file contains the HTTP method, URL (with path parameters as
{{paramName}} placeholders), required headers and query params, an empty JSON
body stub for POST/PUT/PATCH operations, and a Bearer auth hint when the
operation declares a security requirement.

A file that exists with other content is not replaced: nothing is written, the files are listed and
the exit code is 2. --force replaces them. A file that already holds what would be written is left
as it is, and other files in the folder are never touched.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]

			data, specFormat, err := fetchSpec(src)
			if err != nil {
				return err
			}

			spec, err := openapi.ParseBytes(data, specFormat)
			if err != nil {
				return fmt.Errorf("parsing spec: %w", err)
			}

			title, version := spec.Info()
			reqs, err := openapi.GenerateRequests(spec, baseURL)
			if err != nil {
				return fmt.Errorf("generating requests: %w", err)
			}

			// Resolve output directory.
			destDir := outputDir
			if destDir == "" {
				dirName := genSanitizeDirName(title)
				if dirName == "" {
					dirName = "requests"
				}
				destDir = dirName
			}

			type genResult struct {
				OperationID string   `json:"operationId,omitempty"`
				Method      string   `json:"method"`
				URL         string   `json:"url"`
				Path        string   `json:"path"`
				Tags        []string `json:"tags,omitempty"`
				// State is what the file was before this run: new, unchanged or differs
				State importer.FileState `json:"state"`
				data  []byte
			}
			var results []genResult
			var skipped int
			var inTheWay []string
			names := map[string]bool{}

			// every file is looked at before the first is written, so a refusal writes nothing
			for _, gr := range reqs {
				filename := genSanitizeFilename(gr.OperationID)
				if filename == "" {
					filename = genSanitizeFilename(gr.Request.Title)
				}
				if filename == "" {
					filename = strings.ToLower(string(gr.Request.Method)) + "-request"
				}
				savePath := importer.UniqueName(filepath.Join(destDir, filename+".yaml"), names)

				yamlData, err := serialization.WriteRequest(gr.Request, "yaml")
				if err != nil {
					skipped++
					continue
				}
				state, err := importer.StateOf(savePath, yamlData)
				if err != nil {
					return fmt.Errorf("looking at %s: %w", savePath, err)
				}
				if state == importer.FileDiffers {
					inTheWay = append(inTheWay, savePath)
				}

				results = append(results, genResult{
					OperationID: gr.OperationID,
					Method:      string(gr.Request.Method),
					URL:         gr.Request.URL,
					Path:        savePath,
					Tags:        gr.Tags,
					State:       state,
					data:        yamlData,
				})
			}

			if !dryRun {
				if len(inTheWay) > 0 && !force {
					return fmt.Errorf("nothing was written: %d file%s with other content already there:\n  %s\nUse --output with a new folder to keep them, or --force to replace them",
						len(inTheWay), plural(len(inTheWay)), strings.Join(inTheWay, "\n  "))
				}
				written := results[:0]
				for _, r := range results {
					if r.State != importer.FileUnchanged {
						if err := importer.WriteGenerated(r.Path, r.data); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "writing %s: %v\n", r.Path, err)
							skipped++
							continue
						}
					}
					written = append(written, r)
				}
				results = written
			}

			switch strings.ToLower(format) {
			case "json":
				type jsonOut struct {
					SpecTitle   string      `json:"specTitle"`
					SpecVersion string      `json:"specVersion"`
					OutputDir   string      `json:"outputDir"`
					DryRun      bool        `json:"dryRun"`
					Generated   []genResult `json:"generated"`
					Skipped     int         `json:"skipped"`
				}
				b, _ := json.MarshalIndent(jsonOut{
					SpecTitle:   title,
					SpecVersion: version,
					OutputDir:   destDir,
					DryRun:      dryRun,
					Generated:   results,
					Skipped:     skipped,
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			default:
				dryTag := ""
				if dryRun {
					dryTag = " (dry run)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s%s → %s\n\n", title, version, dryTag, destDir)
				for _, r := range results {
					note := ""
					if dryRun && r.State == importer.FileDiffers {
						note = "  (" + r.Path + " has other content: needs --force)"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %-7s %s%s\n", r.Method, r.URL, note)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d request(s) generated", len(results))
				if skipped > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d skipped (write error)", skipped)
				}
				if !dryRun && len(inTheWay) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d replaced (--force)", len(inTheWay))
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&baseURL, "base-url", "b", "", "Override base URL from spec (e.g. https://api.example.com/v1)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for generated files (default: ./<spec-title>)")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be generated without writing any files")
	cmd.Flags().BoolVar(&force, "force", false, "Replace files that exist with other content")

	return cmd
}

// --- End telemetry ---

func genSanitizeDirName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			b.WriteRune('-')
		}
	}
	return b.String()
}

func genSanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune('-')
		}
	}
	return b.String()
}
